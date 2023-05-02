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
	DNSPolicyKind = "DNSPolicy"
)

var (
	DNSPolicyV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         DNSPolicyKind,
	}

	DNSPolicyType = DNSPolicyV1Alpha1Type
)

func RegisterDNSPolicy(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     DNSPolicyV1Alpha1Type,
		Proto:    &pbcatalog.DNSPolicy{},
		Validate: ValidateDNSPolicy,
	})
}

func ValidateDNSPolicy(res *pbresource.Resource) error {
	var policy pbcatalog.DNSPolicy

	if err := res.Data.UnmarshalTo(&policy); err != nil {
		return newErrDataParse(&policy, err)
	}

	var err error
	// Ensure that this resource isn't useless and is attempting to
	// select at least one workload.
	if selErr := validateSelector(policy.Workloads, false); selErr != nil {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	// Validate the weights
	if weightErr := validateDNSPolicyWeights(policy.Weights); weightErr != nil {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "weights",
			Wrapped: weightErr,
		})
	}

	return err
}

func validateDNSPolicyWeights(weights *pbcatalog.Weights) error {
	// Non nil weights are required
	if weights == nil {
		return errMissing
	}

	var err error
	if weights.Passing > 65535 {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "passing",
			Wrapped: errDNSWeightOutOfRange,
		})
	}

	if weights.Warning > 65535 {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "warning",
			Wrapped: errDNSWeightOutOfRange,
		})
	}

	return err
}
