// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"github.com/hashicorp/consul/internal/resource"
	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
)

const (
	HealthStatusKind = "HealthStatus"
)

var (
	HealthStatusV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         HealthStatusKind,
	}

	HealthStatusType = HealthStatusV1Alpha1Type
)

func RegisterHealthStatus(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     HealthStatusV1Alpha1Type,
		Proto:    &pbcatalog.HealthStatus{},
		Validate: ValidateHealthStatus,
	})
}

func ValidateHealthStatus(res *pbresource.Resource) error {
	var hs pbcatalog.HealthStatus

	if err := res.Data.UnmarshalTo(&hs); err != nil {
		return newErrDataParse(&hs, err)
	}

	var err error

	// Should we allow empty types? I think for now it will be safest to require
	// the type field is set and we can relax this restriction in the future
	// if we deem it desirable.
	if hs.Type == "" {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "type",
			Wrapped: errMissing,
		})
	}

	switch hs.Status {
	case pbcatalog.Health_HEALTH_PASSING,
		pbcatalog.Health_HEALTH_WARNING,
		pbcatalog.Health_HEALTH_CRITICAL,
		pbcatalog.Health_HEALTH_MAINTENANCE:
	default:
		err = multierror.Append(err, ErrInvalidField{
			Name:    "status",
			Wrapped: errInvalidHealth,
		})
	}

	// Ensure that the HealthStatus' owner is a type that we want to allow. The
	// owner is currently the resource that this HealthStatus applies to. If we
	// change this to be a parent reference within the HealthStatus.Data then
	// we could allow for other owners.
	if res.Owner == nil {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "owner",
			Wrapped: errMissing,
		})
	} else if !canAssociateHealthStatus(res.Owner) {
		err = multierror.Append(err, ErrOwnerInvalid{ResourceType: res.Id.Type, OwnerType: res.Owner.Type})
	}

	return err
}

func canAssociateHealthStatus(id *pbresource.ID) bool {
	// Ensure that only other resources within this group
	// can own the resource.
	if id.Type.Group != GroupName {
		return false
	}

	// As these checks are performed at admission time we can assume
	// resource version translation will already have occurred and
	// we can enforce that the group version is the current version.
	if id.Type.GroupVersion != CurrentVersion {
		return false
	}

	// ensure the only owners can be Workloads or Nodes
	if id.Type.Kind == WorkloadKind || id.Type.Kind == NodeKind {
		return true
	}

	return false
}
