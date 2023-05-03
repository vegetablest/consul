// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package endpoints

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/controller"
	"github.com/hashicorp/consul/internal/radix"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

// The WorkloadMapper interface is used to provide an implementation around being able to
// map a watch event for a Workload resource and translate it to reconciliation requests
// for all Services that select that workload.
type WorkloadMapper interface {
	// MapWorkloadToServices will take a Workload resource and return controller requests
	// for all Workloads associated with the Node.
	MapWorkloadToServices(ctx context.Context, rt controller.Runtime, res *pbresource.Resource) ([]controller.Request, error)

	// TrackService instructs the WorkloadMapper to associate the given workload
	// ID with the given node ID.
	TrackService(serviceID *pbresource.ID, service *pbcatalog.Service)

	// RemoveServiceTracking will stop the mapper from tracking
	RemoveServiceTracking(serviceID *pbresource.ID)
}

type serviceReqs struct {
	exact  []controller.Request
	prefix []controller.Request
}

type workloadMapper struct {
	lock sync.Mutex
	tree *radix.Tree[*serviceReqs]
}

func DefaultWorkloadMapper() WorkloadMapper {
	return &workloadMapper{
		tree: radix.New[*serviceReqs](),
	}
}

// MapWorkloadToServices will take a Workload resource and return controller requests
// for all Workloads associated with the Node.
func (m *workloadMapper) MapWorkloadToServices(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	m.lock.Lock()
	defer m.lock.Unlock()

	var reqs []controller.Request
	m.tree.WalkPath(res.Id.Name, func(path string, leaf *serviceReqs) bool {
		reqs = append(reqs, leaf.prefix...)

		if path == res.Id.Name {
			reqs = append(reqs, leaf.exact...)
		}
		return false
	})

	return reqs, nil
}

// TrackService instructs the WorkloadMapper to associate the given workload
// ID with the given node ID.
func (m *workloadMapper) TrackService(serviceID *pbresource.ID, service *pbcatalog.Service) {
	m.lock.Lock()
	defer m.lock.Unlock()

	// First we must remove any previous tracking of this service. Doing this first
	// allows us to not have to deal with walking the whole tree and then cross referencing
	// with the current service incarnation
	m.removeServiceTracking(serviceID)

	selector := service.GetWorkloads()

	// loop over all the exact matching rules and associate those workload names with this service
	for _, name := range selector.GetNames() {
		// attempt to lookup any existing tracking information for this workload prefix
		leaf, _ := m.tree.Get(name)
		if leaf == nil {
			// This name/prefix was never previously used so we create a new serviceReqs object
			// to point at our service and insert it into the tree
			m.tree.Insert(name, &serviceReqs{
				exact: []controller.Request{{ID: serviceID}},
			})
		} else {
			// This name/prefix was already being tracked so we can simply append our serviceID
			// to the list of exact match services.
			leaf.exact = append(leaf.exact, controller.Request{ID: serviceID})
		}
	}

	for _, prefix := range selector.GetPrefixes() {
		// attempt to lookup any existing tracking information for this workload prefix
		leaf, _ := m.tree.Get(prefix)
		if leaf == nil {
			// This name/prefix was never previously used so we create a new serviceReqs object
			// to point at our service and insert it into the tree
			m.tree.Insert(prefix, &serviceReqs{
				prefix: []controller.Request{{ID: serviceID}},
			})
		} else {
			// This name/prefix was already being tracked so we can simply append our serviceID
			// to the list of prefix match services.
			leaf.prefix = append(leaf.prefix, controller.Request{ID: serviceID})
		}
	}

	// the initial call to removeServiceTracking could have left empty leaves around
	// this was desirable as it could have allowed us to reuse those leaves and
	// prevent some unnecessary allocations and subsequent GC operations.
	m.pruneEmptyLeaves()
}

// RemoveServiceTracking will stop the mapper from tracking
func (m *workloadMapper) RemoveServiceTracking(serviceID *pbresource.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removeServiceTracking(serviceID)
	m.pruneEmptyLeaves()
}

// removeServiceTracking will erase the given serviceID from all leaf nodes
// within the radix tree. If the given serviceID is the last ID found in a
// particular leaf, this function will NOT remove that leaf entirely. Instead
// it is expected that a call to pruneEmptyLeaves will be made after this.
func (m *workloadMapper) removeServiceTracking(serviceID *pbresource.ID) {
	m.tree.Walk(func(_ string, leaf *serviceReqs) bool {
		foundIdx := -1
		for idx, exact := range leaf.exact {
			// TODO - maybe don't use proto.Equal because it drops to reflection
			// for zero gain here
			if proto.Equal(exact.ID, serviceID) {
				foundIdx = idx
				break
			}
		}

		// We found the Service in this leaf and now must remove it
		if foundIdx != -1 {
			l := len(leaf.exact)

			if l == 1 {
				leaf.exact = nil
			} else if foundIdx == l-1 {
				leaf.exact = leaf.exact[:foundIdx]
			} else if foundIdx == 0 {
				leaf.exact = leaf.exact[1:]
			} else {
				leaf.exact = append(leaf.exact[:foundIdx], leaf.exact[foundIdx+1:]...)
			}
		}

		foundIdx = -1
		for idx, prefix := range leaf.prefix {
			// TODO - maybe don't use proto.Equal because it drops to reflection
			// for zero gain here
			if proto.Equal(prefix.ID, serviceID) {
				foundIdx = idx
				break
			}
		}

		// We found the Service in this leaf and now must remove it
		if foundIdx != -1 {
			l := len(leaf.prefix)

			if l == 1 {
				leaf.prefix = nil
			} else if foundIdx == l-1 {
				leaf.prefix = leaf.prefix[:foundIdx]
			} else if foundIdx == 0 {
				leaf.prefix = leaf.prefix[1:]
			} else {
				leaf.prefix = append(leaf.prefix[:foundIdx], leaf.prefix[foundIdx+1:]...)
			}
		}

		return false
	})
}

func (m *workloadMapper) pruneEmptyLeaves() {
	var toDelete []string
	m.tree.Walk(func(path string, leaf *serviceReqs) bool {
		if len(leaf.exact) < 1 && len(leaf.prefix) < 1 {
			toDelete = append(toDelete, path)
		}
		return false
	})

	for _, path := range toDelete {
		m.tree.Delete(path)
	}
}
