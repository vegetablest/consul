// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package nodehealth

import (
	"context"
	"fmt"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func NodeHealthController() controller.Controller {
	return controller.ForType(types.NodeType).
		WithWatch(types.HealthStatusType, controller.MapOwnerFiltered(types.NodeType)).
		WithReconciler(&nodeHealthReconciler{})
}

type nodeHealthReconciler struct{}

func (r *nodeHealthReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// read the workload
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		return nil
	case err != nil:
		return err
	}

	res := rsp.Resource

	health, err := getNodeHealth(ctx, rt, req.ID)
	if err != nil {
		return err
	}

	message := NodeHealthyMessage
	statusState := pbresource.Condition_STATE_TRUE
	if health != pbcatalog.Health_HEALTH_PASSING {
		statusState = pbresource.Condition_STATE_FALSE
		message = NodeUnhealthyMessage
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
	rsp, err := rt.Client.ListByOwner(ctx, &pbresource.ListByOwnerRequest{
		Owner: nodeRef,
	})

	if err != nil {
		return pbcatalog.Health_HEALTH_CRITICAL, err
	}

	health := pbcatalog.Health_HEALTH_PASSING

	for _, res := range rsp.Resources {
		if proto.Equal(res.Id.Type, types.HealthStatusType) {
			var hs pbcatalog.HealthStatus
			if err := res.Data.UnmarshalTo(&hs); err != nil {
				// This should be impossible as the resource service + type validations the
				// catalog is performing will ensure that no data gets written where unmarshalling
				// to this type will error.
				return health, fmt.Errorf("error unmarshalling health status data: %w", err)
			}

			if hs.Status > health {
				health = hs.Status
			}
		}
	}

	return health, nil
}
