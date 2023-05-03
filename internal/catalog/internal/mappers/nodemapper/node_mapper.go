package nodemapper

import (
	"context"
	"sync"

	"github.com/hashicorp/consul/internal/catalog/internal/types"
	"github.com/hashicorp/consul/internal/controller"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/proto"
)

type NodeMapper struct {
	lock             sync.Mutex
	nodesToWorkloads map[string][]controller.Request
}

func New() *NodeMapper {
	return &NodeMapper{
		nodesToWorkloads: make(map[string][]controller.Request),
	}
}

func (m *NodeMapper) NodeIDFromWorkload(workload *pbresource.Resource, workloadData *pbcatalog.Workload) *pbresource.ID {
	return &pbresource.ID{
		Type:    types.NodeV1Alpha1Type,
		Tenancy: workload.Id.Tenancy,
		Name:    workloadData.NodeName,
	}
}

func (m *NodeMapper) MapNodeToWorkloads(_ context.Context, _ controller.Runtime, res *pbresource.Resource) ([]controller.Request, error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	return m.nodesToWorkloads[res.Id.Name], nil
}

func (m *NodeMapper) TrackWorkload(workloadID *pbresource.ID, nodeID *pbresource.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()

	reqs, ok := m.nodesToWorkloads[nodeID.Name]
	if ok {
		for _, req := range reqs {
			// if the workload already is mapped to the node
			if proto.Equal(req.ID, workloadID) {
				return
			}
		}
	}

	// Check if this workload is being tracked for another node and
	// remove the link. This would only occur if the workloads
	// associated node name is changed.
	m.removeWorkloadTrackingLocked(workloadID)

	// Now set up the latest tracking
	m.nodesToWorkloads[nodeID.Name] = append(reqs, controller.Request{ID: workloadID})
}

func (m *NodeMapper) RemoveWorkloadTracking(workloadID *pbresource.ID) {
	m.lock.Lock()
	defer m.lock.Unlock()
	m.removeWorkloadTrackingLocked(workloadID)
}

func (m *NodeMapper) removeWorkloadTrackingLocked(workloadID *pbresource.ID) {
	// TODO make this not perform in O(<num global workloads>) time
	for existingNodeName, workloads := range m.nodesToWorkloads {
		foundIdx := -1
		for idx, req := range workloads {
			// TODO - maybe don't use proto.Equal because it drops to reflection
			// for zero gain here
			if proto.Equal(req.ID, workloadID) {
				foundIdx = idx
				break
			}
		}

		// We found the Workload tracked by another node name. This means
		// that the Workloads node association is being changed so first
		// we must remove the previous association.
		if foundIdx != -1 {
			l := len(m.nodesToWorkloads[existingNodeName])

			if l == 1 {
				delete(m.nodesToWorkloads, existingNodeName)
			} else if foundIdx == l-1 {
				m.nodesToWorkloads[existingNodeName] = workloads[:foundIdx]
			} else if foundIdx == 0 {
				m.nodesToWorkloads[existingNodeName] = workloads[1:]
			} else {
				m.nodesToWorkloads[existingNodeName] = append(workloads[:foundIdx], workloads[foundIdx+1:]...)
			}

			return
		}
	}
}
