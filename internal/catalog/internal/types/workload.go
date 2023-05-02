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
	WorkloadKind = "Workload"
)

var (
	WorkloadV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         WorkloadKind,
	}

	WorkloadType = WorkloadV1Alpha1Type
)

func RegisterWorkload(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     WorkloadV1Alpha1Type,
		Proto:    &pbcatalog.Workload{},
		Validate: ValidateWorkload,
	})
}

func ValidateWorkload(res *pbresource.Resource) error {
	var workload pbcatalog.Workload

	if err := res.Data.UnmarshalTo(&workload); err != nil {
		return newErrDataParse(&workload, err)
	}

	var err error
	// Only validate the identity if its not empty, if it is
	// empty it will be defaulted elsewhere.
	if workload.Identity != "" {
		if !isValidDNSLabel(workload.Identity) {
			err = multierror.Append(err, ErrInvalidField{
				Name:    "identity",
				Wrapped: errNotDNSLabel,
			})
		}
	}

	// Node associations are optional but if present the name should
	// be a valid DNS label.
	if workload.NodeName != "" {
		if !isValidDNSLabel(workload.NodeName) {
			err = multierror.Append(err, ErrInvalidField{
				Name:    "node_name",
				Wrapped: errNotDNSLabel,
			})
		}
	}

	if len(workload.Addresses) < 1 {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "addresses",
			Wrapped: errEmpty,
		})
	}

	// Validate Workload Addresses
	for idx, addr := range workload.Addresses {
		if addrErr := validateWorkloadAddress(addr, workload.Ports); addrErr != nil {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:    "addresses",
				Index:   idx,
				Wrapped: addrErr,
			})
		}
	}

	// Validate that the workload has at least one port
	if len(workload.Ports) < 1 {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "ports",
			Wrapped: errEmpty,
		})
	}

	// Validate the Workload Ports
	for portName, port := range workload.Ports {
		if portNameErr := validatePortName(portName); portNameErr != nil {
			err = multierror.Append(err, ErrInvalidMapKey{
				Map:     "ports",
				Key:     portName,
				Wrapped: portNameErr,
			})
		}

		// disallow port 0 for now
		if port.Port < 1 || port.Port > 65535 {
			err = multierror.Append(err, ErrInvalidMapValue{
				Map: "ports",
				Key: portName,
				Wrapped: ErrInvalidField{
					Name:    "port",
					Wrapped: errInvalidPhysicalPort,
				},
			})
		}
	}

	// Validate workload locality
	if workload.Locality != nil && workload.Locality.Region == "" && workload.Locality.Zone != "" {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "locality",
			Wrapped: errLocalityZoneNoRegion,
		})
	}

	return err
}

func MutateWorkload(res *pbresource.Resource) error {
	var workload pbcatalog.Workload
	modified := false

	if err := res.Data.UnmarshalTo(&workload); err != nil {
		return newErrDataParse(&workload, err)
	}

	// default the workload identity to the workloads name if none was provided
	if workload.Identity == "" {
		workload.Identity = res.Id.Name
		modified = true
	}

	// If we modified the workload then we must marshal the data again
	if modified {
		return res.Data.MarshalFrom(&workload)
	}

	return nil
}
