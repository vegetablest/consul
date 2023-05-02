// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"fmt"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	errMissing                  = errors.New("missing required field")
	errEmpty                    = errors.New("cannot be empty")
	errNotDNSLabel              = errors.New(fmt.Sprintf("value must match regex: %s", dnsLabelRegex))
	errNotIPAddress             = errors.New("value is not a valid IP address")
	errUnixSocketMultiport      = errors.New("Unix socket address references more than one port")
	errInvalidPhysicalPort      = errors.New("port number is outside the range 1 to 65535")
	errInvalidVirtualPort       = errors.New("port number is outside the range 0 to 65535")
	errDNSWeightOutOfRange      = errors.New("DNS weight is outside the range 0 to 65535")
	errLocalityZoneNoRegion     = errors.New("locality region cannot be empty if the zone is set")
	errReferenceTenancyNotEqual = errors.New("resource tenancy and reference tenancy differ")
	errInvalidHealth            = errors.New("health status must be one of: passing, warning, critical or maintenance")
)

type ErrDataParse struct {
	TypeName string
	Wrapped  error
}

func newErrDataParse(msg protoreflect.ProtoMessage, err error) ErrDataParse {
	return ErrDataParse{
		TypeName: string(msg.ProtoReflect().Descriptor().FullName()),
		Wrapped:  err,
	}
}

func (err ErrDataParse) Error() string {
	return fmt.Sprintf("error parsing resource data as type %q: %s", err.TypeName, err.Wrapped.Error())
}

func (err ErrDataParse) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidField struct {
	Name    string
	Wrapped error
}

func (err ErrInvalidField) Error() string {
	return fmt.Sprintf("invalid %s field: %v", err.Name, err.Wrapped)
}

func (err ErrInvalidField) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidListElement struct {
	Name    string
	Index   int
	Wrapped error
}

func (err ErrInvalidListElement) Error() string {
	return fmt.Sprintf("invalid element at index %d of list %q: %v", err.Index, err.Name, err.Wrapped)
}

func (err ErrInvalidListElement) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidMapValue struct {
	Map     string
	Key     string
	Wrapped error
}

func (err ErrInvalidMapValue) Error() string {
	return fmt.Sprintf("invalid value of key %q within %s: %v", err.Key, err.Map, err.Wrapped)
}

func (err ErrInvalidMapValue) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidMapKey struct {
	Map     string
	Key     string
	Wrapped error
}

func (err ErrInvalidMapKey) Error() string {
	return fmt.Sprintf("map %s contains an invalid key - %q: %v", err.Key, err.Map, err.Wrapped)
}

func (err ErrInvalidMapKey) Unwrap() error {
	return err.Wrapped
}

type ErrInvalidWorkloadHostFormat struct {
	Host string
}

func (err ErrInvalidWorkloadHostFormat) Error() string {
	return fmt.Sprintf("%q is not an IP address, Unix socket path or a DNS name.", err.Host)
}

type ErrInvalidNodeHostFormat struct {
	Host string
}

func (err ErrInvalidNodeHostFormat) Error() string {
	return fmt.Sprintf("%q is not an IP address or a DNS name.", err.Host)
}

type ErrOwnerInvalid struct {
	ResourceType *pbresource.Type
	OwnerType    *pbresource.Type
}

func (err ErrOwnerInvalid) Error() string {
	return fmt.Sprintf(
		"resources of type %s.%s.%s cannot be owned by resources with type %s.%s.%s",
		err.ResourceType.Group, err.ResourceType.GroupVersion, err.ResourceType.Kind,
		err.OwnerType.Group, err.OwnerType.GroupVersion, err.OwnerType.Kind,
	)
}

type ErrInvalidPortReference struct {
	Name string
}

func (err ErrInvalidPortReference) Error() string {
	return fmt.Sprintf("port with name %q has not been defined", err.Name)
}

type ErrInvalidReferenceType struct {
	AllowedType *pbresource.Type
}

func (err ErrInvalidReferenceType) Error() string {
	return fmt.Sprintf("reference must have type %s.%s.%s",
		err.AllowedType.Group,
		err.AllowedType.GroupVersion,
		err.AllowedType.Kind)
}

type ErrVirtualPortReused struct {
	Index int
	Value uint32
}

func (err ErrVirtualPortReused) Error() string {
	return fmt.Sprintf("virtual port %d was previously assigned at index %d", err.Value, err.Index)
}
