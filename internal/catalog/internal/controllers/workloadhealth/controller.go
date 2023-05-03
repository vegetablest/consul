package workloadhealth

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/nodehealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

var (
	errNodeUnreconciled            = errors.New("Node health has not been reconciled yet")
	errNodeHealthInvalid           = errors.New("Node health has invalid reason")
	errNodeHealthConditionNotFound = fmt.Errorf("Node health status is missing the %s condition", nodehealth.StatusConditionHealthy)
)

func WorkloadHealthController(nodeMap NodeMapper) controller.Controller {
	if nodeMap == nil {
		panic("No NodeMapper was provided to the WorkloadHealthController constructor")
	}

	return controller.ForType(types.WorkloadType).
		WithWatch(types.HealthStatusType, controller.MapOwnerFiltered(types.WorkloadType)).
		WithWatch(types.NodeType, nodeMap.MapNodeToWorkloads).
		WithReconciler(&workloadHealthReconciler{nodeMap: nodeMap})
}

type workloadHealthReconciler struct {
	nodeMap NodeMapper
}

func (r *workloadHealthReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// read the workload
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		r.nodeMap.RemoveWorkloadTracking(req.ID)
		return nil
	case err != nil:
		return err
	}

	res := rsp.Resource
	var workload pbcatalog.Workload
	if err := res.Data.UnmarshalTo(&workload); err != nil {
		// This should be impossible and will not be exercised in tests. Various
		// type validations on admission ensure that all Workloads would
		// be unmarshallable in this way.
		return err
	}

	nodeHealth := pbcatalog.Health_HEALTH_PASSING
	if workload.NodeName != "" {
		nodeID := r.nodeMap.NodeIDFromWorkload(res, &workload)
		r.nodeMap.TrackWorkload(res.Id, nodeID)
		nodeHealth, err = getNodeHealth(ctx, rt, nodeID)
		if err != nil {
			return err
		}
	} else {
		// the node association may be been removed so stop tracking it.
		r.nodeMap.RemoveWorkloadTracking(res.Id)
	}

	workloadHealth, err := getWorkloadHealth(ctx, rt, req.ID)
	if err != nil {
		// This should be impossible under normal operations and will not be exercised
		// within the unit tests. This can only fail if the resource service fails
		// or allows admission of invalid health statuses.
		return err
	}

	health := nodeHealth
	if workloadHealth > health {
		health = workloadHealth
	}

	statusState := pbresource.Condition_STATE_TRUE
	if health != pbcatalog.Health_HEALTH_PASSING {
		statusState = pbresource.Condition_STATE_FALSE
	}

	message := WorkloadHealthyMessage
	if workload.NodeName != "" {
		message = NodeAndWorkloadHealthyMessage
	}
	switch {
	case workloadHealth != pbcatalog.Health_HEALTH_PASSING && nodeHealth != pbcatalog.Health_HEALTH_PASSING:
		message = NodeAndWorkloadUnhealthyMessage
	case workloadHealth != pbcatalog.Health_HEALTH_PASSING:
		message = WorkloadUnhealthyMessage
	case nodeHealth != pbcatalog.Health_HEALTH_PASSING:
		message = nodehealth.NodeUnhealthyMessage
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: res.Generation,
		Conditions: []*pbresource.Condition{
			{
				Type:    StatusConditionHealthy,
				State:   statusState,
				Reason:  health.String(),
				Message: message,
			},
		},
	}

	if proto.Equal(res.Status[StatusKey], newStatus) {
		return nil
	}

	_, err = rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     res.Id,
		Key:    StatusKey,
		Status: newStatus,
	})

	return err
}

func getNodeHealth(ctx context.Context, rt controller.Runtime, nodeRef *pbresource.ID) (pbcatalog.Health, error) {
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: nodeRef})
	switch {
	case status.Code(err) == codes.NotFound:
		return pbcatalog.Health_HEALTH_CRITICAL, nil
	case err != nil:
		return pbcatalog.Health_HEALTH_CRITICAL, err
	default:
		healthStatus, ok := rsp.Resource.Status[nodehealth.StatusKey]
		if !ok {
			// The Nodes health has never been reconciled and therefore the
			// workloads health cannot be determined. Returning nil is acceptable
			// because the controller should sometime soon run reconciliation for
			// the node which will then trigger rereconciliation of this workload
			return pbcatalog.Health_HEALTH_CRITICAL, errNodeUnreconciled
		}

		for _, condition := range healthStatus.Conditions {
			if condition.Type == nodehealth.StatusConditionHealthy {
				if condition.State == pbresource.Condition_STATE_TRUE {
					return pbcatalog.Health_HEALTH_PASSING, nil
				}

				healthReason, valid := pbcatalog.Health_value[condition.Reason]
				if !valid {
					// The Nodes health is unknown - presumably the node health controller
					// will come along and fix that up momentarily causing this workload
					// reconciliation to occur again.
					return pbcatalog.Health_HEALTH_CRITICAL, errNodeHealthInvalid
				}
				return pbcatalog.Health(healthReason), nil
			}
		}
		return pbcatalog.Health_HEALTH_CRITICAL, errNodeHealthConditionNotFound
	}
}

func getWorkloadHealth(ctx context.Context, rt controller.Runtime, workloadRef *pbresource.ID) (pbcatalog.Health, error) {
	rsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{
		Owner: workloadRef,
	})

	if err != nil {
		return pbcatalog.Health_HEALTH_CRITICAL, err
	}

	workloadHealth := pbcatalog.Health_HEALTH_PASSING

	for _, res := range rsp.Resources {
		if proto.Equal(res.Id.Type, types.HealthStatusType) {
			var hs pbcatalog.HealthStatus
			if err := res.Data.UnmarshalTo(&hs); err != nil {
				// This should be impossible and will not be executing in tests. The resource type
				// is the HealthStatus type and therefore must be unmarshallable into the HealthStatus
				// object or else it wouldn't have passed admission validation checks.
				return workloadHealth, fmt.Errorf("error unmarshalling health status data: %w", err)
			}

			if hs.Status > workloadHealth {
				workloadHealth = hs.Status
			}
		}
	}

	return workloadHealth, nil
}
