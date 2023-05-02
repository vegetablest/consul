// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"net"
	"regexp"
	"strings"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/hashicorp/go-multierror"
	"google.golang.org/protobuf/proto"
)

var (
	dnsLabelRegex   = `^[a-z0-9]([a-z0-9\-_]*[a-z0-9])?$`
	dnsLabelMatcher = regexp.MustCompile(dnsLabelRegex)
)

func isValidIPAddress(host string) bool {
	return net.ParseIP(host) != nil
}

func isValidDNSName(host string) bool {
	if len(host) > 256 {
		return false
	}

	labels := strings.Split(host, ".")
	for _, label := range labels {
		if !isValidDNSLabel(label) {
			return false
		}
	}

	return true
}

func isValidDNSLabel(label string) bool {
	if len(label) > 64 {
		return false
	}

	return dnsLabelMatcher.Match([]byte(label))
}

func isValidUnixSocketPath(host string) bool {
	if !strings.HasPrefix(host, "unix://") || strings.Contains(host, "\000") {
		return false
	}

	return true
}

func validateWorkloadHost(host string) error {
	// Check that the host is empty
	if host == "" {
		return ErrInvalidWorkloadHostFormat{Host: host}
	}

	// Check if the host represents an IP address, unix socket path or a DNS name
	if !isValidIPAddress(host) && !isValidUnixSocketPath(host) && !isValidDNSName(host) {
		return ErrInvalidWorkloadHostFormat{Host: host}
	}

	return nil
}

func validateSelector(sel *pbcatalog.WorkloadSelector, allowEmpty bool) error {
	if sel == nil {
		if allowEmpty {
			return nil
		}

		return errEmpty
	}

	if len(sel.Names) == 0 && len(sel.Prefixes) == 0 {
		if allowEmpty {
			return nil
		}

		return errEmpty
	}

	var err error

	// Validate that all the exact match names are non-empty. This is
	// mostly for the sake of not admitting values that should always
	// be meaningless and never actually cause selection of a workload.
	// This is because workloads must have non-empty names.
	for idx, name := range sel.Names {
		if name == "" {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:    "names",
				Index:   idx,
				Wrapped: errEmpty,
			})
		}
	}

	return err
}

func validateIPAddress(ip string) error {
	if ip == "" {
		return errEmpty
	}

	if !isValidIPAddress(ip) {
		return errNotIPAddress
	}

	return nil
}

func validatePortName(name string) error {
	if name == "" {
		return errEmpty
	}

	if !isValidDNSLabel(name) {
		return errNotDNSLabel
	}

	return nil
}

// generics here are so that the ports map can have either the WorkloadPort or EndpointPort
// types. We don't actually need the values in this function but using generics prevents
// needing to either duplicate the code or to convert between map types
func validateWorkloadAddress[T any](addr *pbcatalog.WorkloadAddress, ports map[string]T) error {
	var err error

	if hostErr := validateWorkloadHost(addr.Host); hostErr != nil {
		err = multierror.Append(err, ErrInvalidField{
			Name:    "host",
			Wrapped: hostErr,
		})
	}

	// Ensure that unix sockets reference exactly 1 port. They may also indirectly reference 1 port
	// by the workload having only a single port and omitting any explicit port assignment.
	if isValidUnixSocketPath(addr.Host) &&
		(len(addr.Ports) > 1 || (len(addr.Ports) == 0 && len(ports) > 1)) {
		err = multierror.Append(err, errUnixSocketMultiport)
	}

	// Check that all referenced ports exist
	for idx, port := range addr.Ports {
		_, found := ports[port]
		if !found {
			err = multierror.Append(err, ErrInvalidListElement{
				Name:    "ports",
				Index:   idx,
				Wrapped: ErrInvalidPortReference{Name: port},
			})
		}
	}
	return err
}

func validateReferenceType(allowed *pbresource.Type, check *pbresource.Type) error {
	if allowed.Group == check.Group &&
		allowed.GroupVersion == check.GroupVersion &&
		allowed.Kind == check.Kind {
		return nil
	}

	return ErrInvalidReferenceType{
		AllowedType: allowed,
	}
}

func validateReferenceTenancy(allowed *pbresource.Tenancy, check *pbresource.Tenancy) error {
	if proto.Equal(allowed, check) {
		return nil
	}

	return errReferenceTenancyNotEqual
}

func validateReference(allowedType *pbresource.Type, allowedTenancy *pbresource.Tenancy, check *pbresource.ID) error {
	var err error

	// Validate the references type is the allowed type.
	if typeErr := validateReferenceType(allowedType, check.GetType()); typeErr != nil {
		err = multierror.Append(err, typeErr)
	}

	// Validate the references tenancy matches the allowed tenancy.
	if tenancyErr := validateReferenceTenancy(allowedTenancy, check.GetTenancy()); tenancyErr != nil {
		err = multierror.Append(err, tenancyErr)
	}

	return err
}
