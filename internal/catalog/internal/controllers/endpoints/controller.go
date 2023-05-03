// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package endpoints

import (
	"context"
	"sort"

	"github.com/hashicorp/consul/internal/catalog/internal/controllers/workloadhealth"
	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	endpointsMetaManagedBy = "managed-by"
)

func ServiceEndpointsController(workloadMap WorkloadMapper) controller.Controller {
	if workloadMap == nil {
		panic("No WorkloadMapper was provided to the ServiceEndpointsController constructor")
	}

	return controller.ForType(types.ServiceType).
		WithWatch(types.ServiceEndpointsType, controller.MapOwner).
		WithWatch(types.WorkloadType, workloadMap.MapWorkloadToServices).
		WithReconciler(newServiceEndpointsReconciler(workloadMap))
}

type serviceEndpointsReconciler struct {
	workloadMap WorkloadMapper

	// We keep track of which services are being managed so that we can know
	// when one transitions from being managed to unmanaged and delete the
	// corresponding ServiceEndpoints accordingly.
	managedServices map[string]struct{}
}

func newServiceEndpointsReconciler(workloadMap WorkloadMapper) *serviceEndpointsReconciler {
	return &serviceEndpointsReconciler{
		workloadMap:     workloadMap,
		managedServices: make(map[string]struct{}),
	}
}

// reconcileContext is a small struct to hold onto the varous bits of
// data needed commonly amongst many of the functions that the reconciler
// will call. This prevents needing to pass all these values through most
// of the call stack.
//
// The notable exception is the context.Context but the docs for that
// advise against storing those in any structure.
type reconcileContext struct {
	rt          controller.Runtime
	resource    *pbresource.Resource
	service     *pbcatalog.Service
	endpointsID *pbresource.ID
}

func (r *serviceEndpointsReconciler) Reconcile(ctx context.Context, rt controller.Runtime, req controller.Request) error {
	// read the service
	rsp, err := rt.Client.Read(ctx, &pbresource.ReadRequest{Id: req.ID})
	switch {
	case status.Code(err) == codes.NotFound:
		// The service was deleted so we need to update the WorkloadMapper to tell it to
		// stop tracking this service
		r.workloadMap.RemoveServiceTracking(req.ID)

		// Note that because we configured ServiceEndpoints to be owned by the service,
		// the service endpoints object should eventually be automatically deleted.
		// There is no reason to attempt deletion here.
		return nil
	case err != nil:
		return err
	}

	// Pull the Service out of the Resources Any data
	serviceRes := rsp.Resource
	var service pbcatalog.Service
	if err := serviceRes.Data.UnmarshalTo(&service); err != nil {
		return err
	}

	rctx := reconcileContext{
		rt:       rt,
		resource: serviceRes,
		service:  &service,
		endpointsID: &pbresource.ID{
			Type:    types.ServiceEndpointsType,
			Tenancy: serviceRes.Id.Tenancy,
			Name:    serviceRes.Id.Name,
		},
	}

	underManagement := serviceUnderManagement(&rctx)
	if !underManagement {
		// Inform the WorkloadMapper that it no longer needs to track this service
		// as its no longer under endpoint management
		r.workloadMap.RemoveServiceTracking(serviceRes.Id)

		err := r.maybeDeleteServiceEndpoints(ctx, &rctx)
		if err != nil {
			return err
		}
	} else {
		// Inform the WorkloadMapper to track this service and its selectors. So
		// future workload updates that would be matched by the services selectors
		// cause this service to be rereconciled.
		r.workloadMap.TrackService(serviceRes.Id, &service)

		err := r.maybeUpsertServiceEndpoints(ctx, &rctx)
		if err != nil {
			return err
		}
	}

	return r.setServiceStatus(ctx, &rctx, underManagement)
}

func (r *serviceEndpointsReconciler) maybeDeleteServiceEndpoints(ctx context.Context, rctx *reconcileContext) error {
	shouldDelete, err := r.shouldDeleteUnmanagedServiceEndpoints(ctx, rctx)
	if err != nil {
		return err
	}

	if shouldDelete {
		// delete the previously managed endpoints - after this the user may
		// manually manage the ServiceEndpoints object corresponding to the Service
		_, err := rctx.rt.Client.Delete(ctx, &pbresource.DeleteRequest{
			Id: rctx.endpointsID,
		})

		if err != nil {
			return err
		}

		delete(r.managedServices, rctx.resource.Id.Name)
	}

	return nil
}

func (r *serviceEndpointsReconciler) maybeUpsertServiceEndpoints(ctx context.Context, rctx *reconcileContext) error {
	// First we need to gather all the workloads selected by the service
	workloads, err := gatherWorkloadsForService(ctx, rctx)
	if err != nil {
		return err
	}

	// Now all the gathered workloads must be converted to endpoints
	newEndpoints := workloadsToEndpoints(rctx, workloads)

	shouldUpdate, err := endpointsNeedUpdating(ctx, rctx, newEndpoints)
	if err != nil {
		return err
	}

	if shouldUpdate {
		data, err := anypb.New(newEndpoints)
		if err != nil {
			return err
		}
		_, err = rctx.rt.Client.Write(ctx, &pbresource.WriteRequest{
			Resource: &pbresource.Resource{
				Id:    rctx.endpointsID,
				Owner: rctx.resource.Id,
				Metadata: map[string]string{
					endpointsMetaManagedBy: StatusKey,
				},
				Data: data,
			},
		})
		if err != nil {
			return err
		}

		r.managedServices[rctx.resource.Id.Name] = struct{}{}
	}

	return nil
}

func (r *serviceEndpointsReconciler) setServiceStatus(ctx context.Context, rctx *reconcileContext, managed bool) error {
	statusState := pbresource.Condition_STATE_FALSE
	reason := StatusReasonSelectorNotFound
	message := SelectorNotFoundMessage
	if managed {
		statusState = pbresource.Condition_STATE_TRUE
		reason = StatusReasonSelectorFound
		message = SelectorFoundMessage
	}

	newStatus := &pbresource.Status{
		ObservedGeneration: rctx.resource.Generation,
		Conditions: []*pbresource.Condition{
			{
				Type:    StatusConditionAccepted,
				State:   pbresource.Condition_STATE_TRUE,
				Reason:  StatusReasonPassedValidation,
				Message: PassedValidationMessage,
			},
			{
				Type:    StatusConditionEndpointsManaged,
				State:   statusState,
				Reason:  reason,
				Message: message,
			},
		},
	}

	if proto.Equal(rctx.resource.Status[StatusKey], newStatus) {
		return nil
	}

	_, err := rctx.rt.Client.WriteStatus(ctx, &pbresource.WriteStatusRequest{
		Id:     rctx.resource.Id,
		Key:    StatusKey,
		Status: newStatus,
	})

	return err
}

func (r *serviceEndpointsReconciler) shouldDeleteUnmanagedServiceEndpoints(ctx context.Context, rctx *reconcileContext) (bool, error) {
	if _, previouslyManaged := r.managedServices[rctx.resource.Id.Name]; previouslyManaged {
		return true, nil
	}

	// if we didn't previously manage things they we should check the metadata on the resource
	rsp, err := rctx.rt.Client.Read(ctx, &pbresource.ReadRequest{
		Id: rctx.endpointsID,
	})
	if err != nil {
		return false, err
	}

	val, ok := rsp.Resource.Metadata[endpointsMetaManagedBy]
	return ok && val == "true", nil
}

func gatherWorkloadsForService(ctx context.Context, rctx *reconcileContext) ([]*pbresource.Resource, error) {
	var workloads []*pbresource.Resource

	sel := rctx.service.GetWorkloads()

	// this map will track all the gathered workloads by name, this is mainly to deduplicate workloads if they
	// are specified multiple times throughout the list of selection criteria
	workloadNames := make(map[string]struct{})

	// Gather the exact match selections first
	for _, name := range sel.Names {
		workload, err := getWorkloadByName(ctx, rctx, name)
		if err != nil {
			return nil, err
		}

		if workload != nil {
			if _, found := workloadNames[workload.Id.Name]; !found {
				workloads = append(workloads, workload)
				workloadNames[workload.Id.Name] = struct{}{}
			}
		}
	}

	// Now gather all the prefix matched workloads
	for _, prefix := range sel.Prefixes {
		res, err := getWorkloadsByPrefix(ctx, rctx, prefix)
		if err != nil {
			return nil, err
		}

		for _, workload := range res {
			if _, found := workloadNames[workload.Id.Name]; !found {
				workloads = append(workloads, workload)
				workloadNames[workload.Id.Name] = struct{}{}
			}
		}
	}

	// Sorting ensures deterministic output. This will help for testing but
	// the real reason to do this is so we will be able to diff the set of
	// workloads endpoints to determine if we need to update them.
	sort.Slice(workloads, func(i, j int) bool {
		return workloads[i].Id.Name < workloads[j].Id.Name
	})

	return workloads, nil
}

func getWorkloadByName(ctx context.Context, rctx *reconcileContext, name string) (*pbresource.Resource, error) {
	rsp, err := rctx.rt.Client.Read(ctx, &pbresource.ReadRequest{
		Id: &pbresource.ID{
			Type:    types.WorkloadType,
			Tenancy: rctx.resource.Id.Tenancy,
			Name:    name,
		},
	})
	if status.Code(err) == codes.NotFound {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return rsp.Resource, nil
}

func getWorkloadsByPrefix(ctx context.Context, rctx *reconcileContext, namePrefix string) ([]*pbresource.Resource, error) {
	rsp, err := rctx.rt.Client.List(ctx, &pbresource.ListRequest{
		Type:       types.WorkloadType,
		Tenancy:    rctx.resource.Id.Tenancy,
		NamePrefix: namePrefix,
	})
	if err != nil {
		return nil, err
	}

	return rsp.Resources, nil
}

func determineWorkloadHealth(workload *pbresource.Resource) pbcatalog.Health {
	// loop over all workload status conditions for the workload health controller
	// If it hasn't been reconciled the bod of the loop will not actually execute
	// and we will instead return the ANY status.
	for _, condition := range workload.Status[workloadhealth.StatusKey].Conditions {
		if condition.Type == workloadhealth.StatusConditionHealthy {
			return pbcatalog.Health(pbcatalog.Health_value[condition.Reason])
		}
	}
	return pbcatalog.Health_HEALTH_ANY
}

func serviceUnderManagement(rctx *reconcileContext) bool {
	sel := rctx.service.GetWorkloads()
	if sel == nil {
		// The selector wasn't present at all. Therefore this service is not under
		// automatic endpoint management.
		return false
	}

	if len(sel.Names) < 1 && len(sel.Prefixes) < 1 {
		// The selector was set in the request but the list of workload names
		// and prefixes were both empty. Therefore this service is not under
		// automatic endpoint management
		return false
	}

	// Some workload selection criteria exists, so this service is consider
	// under automatic endpoint management.
	return true
}

func endpointsNeedUpdating(ctx context.Context, rctx *reconcileContext, newEndpoints *pbcatalog.ServiceEndpoints) (bool, error) {
	rsp, err := rctx.rt.Client.Read(ctx, &pbresource.ReadRequest{
		Id: rctx.endpointsID,
	})

	if status.Code(err) == codes.NotFound {
		return true, nil
	}

	if err != nil {
		return false, err
	}

	var currentEndpoints pbcatalog.ServiceEndpoints
	err = rsp.Resource.Data.UnmarshalTo(&currentEndpoints)
	if err != nil {
		return false, err
	}

	return !proto.Equal(&currentEndpoints, newEndpoints), nil
}

func workloadsToEndpoints(rctx *reconcileContext, workloads []*pbresource.Resource) *pbcatalog.ServiceEndpoints {
	var endpoints []*pbcatalog.Endpoint

	for _, workload := range workloads {
		endpoint := workloadToEndpoint(rctx, workload)
		if endpoint != nil {
			endpoints = append(endpoints, endpoint)
		}
	}

	return &pbcatalog.ServiceEndpoints{
		Endpoints: endpoints,
	}
}

func workloadToEndpoint(rctx *reconcileContext, res *pbresource.Resource) *pbcatalog.Endpoint {
	// Converting a workload to an endpoint is slightly more complex then it might appear at first
	// glance. We can determine the overall health from the Status of the resource as we can
	// count on the workload health controller eventually adding that. Where things are more nuanced
	// is with calculating the workloads addresses and ports. First is the fact that the ports in
	// the endpoint objects are endpoint ports and not workload ports. The names in the map should
	// be the service port names but the contents should include a mix of data from the service
	// (virtual port) the workload (real port) and both (protocol - it must match between both).

	health := determineWorkloadHealth(res)

	var workload pbcatalog.Workload
	err := res.Data.UnmarshalTo(&workload)
	if err != nil {
		return nil
	}

	endpointPorts := make(map[string]*pbcatalog.WorkloadPort)

	portTranslation := make(map[string]string)

	for _, svcPort := range rctx.service.Ports {
		workloadPort, found := workload.Ports[svcPort.TargetPort]
		if !found {
			// this workload doesn't have this port so ignore it
			continue
		}

		if workloadPort.Protocol != svcPort.Protocol {
			// workload port mismatch - ignore it
			continue
		}

		endpointPorts[svcPort.TargetPort] = &pbcatalog.WorkloadPort{
			Port:     workloadPort.Port,
			Protocol: svcPort.Protocol,
		}

		// Keeping this map assumes there is a 1:1 or 0:1 relationship between
		// service ports and workload ports. (i.e. two service ports cannot select
		// the same target workload port)
		portTranslation[svcPort.TargetPort] = svcPort.TargetPort
	}

	var workloadAddrs []*pbcatalog.WorkloadAddress
	for _, addr := range workload.Addresses {
		var ports []string

		if len(addr.Ports) > 0 {
			for _, portName := range addr.Ports {
				svcPort, found := portTranslation[portName]
				if !found {
					// this port isn't selected by the service so
					// drop this port
					continue
				}

				ports = append(ports, svcPort)
			}
		} else {
			for _, svcPort := range portTranslation {
				ports = append(ports, svcPort)
			}
		}

		if len(ports) > 0 {
			workloadAddrs = append(workloadAddrs, &pbcatalog.WorkloadAddress{
				Host:     addr.Host,
				External: addr.External,
				Ports:    ports,
			})
		}
	}

	if len(workloadAddrs) < 1 {
		return nil
	}

	return &pbcatalog.Endpoint{
		TargetRef:    res.Id,
		HealthStatus: health,
		Addresses:    workloadAddrs,
		Ports:        endpointPorts,
	}
}
