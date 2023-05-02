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
	ServiceKind = "Service"
)

var (
	ServiceV1Alpha1Type = &pbresource.Type{
		Group:        GroupName,
		GroupVersion: VersionV1Alpha1,
		Kind:         ServiceKind,
	}

	ServiceType = ServiceV1Alpha1Type
)

func RegisterService(r resource.Registry) {
	r.Register(resource.Registration{
		Type:     ServiceV1Alpha1Type,
		Proto:    &pbcatalog.Service{},
		Validate: ValidateService,
	})
}

func ValidateService(res *pbresource.Resource) error {
	var service pbcatalog.Service

	if err := res.Data.UnmarshalTo(&service); err != nil {
		return newErrDataParse(&service, err)
	}

	var err error

	// Validate the workload selector. We are allowing selectors with no
	// selection criteria as it will allow for users to manually control
	// manage ServiceEndpoints objects for this service such as when
	// desiring to not endpoint information for external services.
	if selErr := validateSelector(service.Workloads, true); selErr != nil {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "workloads",
			Wrapped: selErr,
		})
	}

	usedVirtualPorts := make(map[uint32]int)

	// Validate each port
	for idx, port := range service.Ports {
		if usedIdx, found := usedVirtualPorts[port.VirtualPort]; found {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: ErrInvalidField{
					Name: "virtual_port",
					Wrapped: ErrVirtualPortReused{
						Index: usedIdx,
						Value: port.VirtualPort,
					},
				},
			})
		} else {
			usedVirtualPorts[port.VirtualPort] = idx
		}

		// validate the target port
		if nameErr := validatePortName(port.TargetPort); nameErr != nil {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:  "ports",
				Index: idx,
				Wrapped: ErrInvalidField{
					Name:    "target_port",
					Wrapped: nameErr,
				},
			})
		}

		// basic protobuf deserialization should enforce that only known variants of the protocol field are set.
	}

	// Validate that the Virtual IPs are all IP addresses
	for idx, vip := range service.VirtualIps {
		if vipErr := validateIPAddress(vip); vipErr != nil {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:    "virtual_ips",
				Index:   idx,
				Wrapped: vipErr,
			})
		}
	}

	return err
}
