// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"testing"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func createWorkloadResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: WorkloadType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "api-1234",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func validWorkload() *pbcatalog.Workload {
	return &pbcatalog.Workload{
		Addresses: []*pbcatalog.WorkloadAddress{
			{
				Host: "127.0.0.1",
			},
		},
		Ports: map[string]*pbcatalog.WorkloadPort{
			"http": {
				Port:     8443,
				Protocol: pbcatalog.Protocol_PROTOCOL_HTTP2,
			},
		},
		NodeName: "foo",
		Identity: "api",
		Locality: &pbcatalog.Locality{
			Region: "us-east-1",
			Zone:   "1a",
		},
	}
}

func TestValidateWorkload_Ok(t *testing.T) {
	res := createWorkloadResource(t, validWorkload())

	err := ValidateWorkload(res)
	require.NoError(t, err)
}

func TestValidateWorkload_ParseError(t *testing.T) {
	// Any type other than the Workload type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	require.True(t, errors.As(err, &ErrDataParse{}))
}

func TestValidateWorkload_EmptyAddresses(t *testing.T) {
	data := validWorkload()
	data.Addresses = nil

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "addresses",
		Wrapped: errEmpty,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidAddress(t *testing.T) {
	data := validWorkload()
	data.Addresses[0].Host = "-not-a-host"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "host",
		Wrapped: ErrInvalidWorkloadHostFormat{Host: "-not-a-host"},
	}

	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidIdentity(t *testing.T) {
	data := validWorkload()
	data.Identity = "/foiujd"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "identity",
		Wrapped: errNotDNSLabel,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidNodeName(t *testing.T) {
	data := validWorkload()
	data.NodeName = "/foiujd"

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "node_name",
		Wrapped: errNotDNSLabel,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_NoPorts(t *testing.T) {
	data := validWorkload()
	data.Ports = nil

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "ports",
		Wrapped: errEmpty,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_InvalidPortName(t *testing.T) {
	data := validWorkload()
	data.Ports[""] = &pbcatalog.WorkloadPort{
		Port: 42,
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidMapKey{
		Map:     "ports",
		Key:     "",
		Wrapped: errEmpty,
	}
	var actual ErrInvalidMapKey
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_Port0(t *testing.T) {
	data := validWorkload()
	data.Ports["bar"] = &pbcatalog.WorkloadPort{Port: 0}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "port",
		Wrapped: errInvalidPhysicalPort,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_PortTooHigh(t *testing.T) {
	data := validWorkload()
	data.Ports["bar"] = &pbcatalog.WorkloadPort{Port: 65536}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "port",
		Wrapped: errInvalidPhysicalPort,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateWorkload_Locality(t *testing.T) {
	data := validWorkload()
	data.Locality = &pbcatalog.Locality{
		Zone: "1a",
	}

	res := createWorkloadResource(t, data)

	err := ValidateWorkload(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "locality",
		Wrapped: errLocalityZoneNoRegion,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestMutateWorkload_Mutated(t *testing.T) {
	data := validWorkload()
	data.Identity = ""

	res := createWorkloadResource(t, data)

	require.NoError(t, MutateWorkload(res))

	var newData pbcatalog.Workload
	require.NoError(t, res.Data.UnmarshalTo(&newData))

	require.Equal(t, res.Id.Name, newData.Identity)
}

func TestMutateWorkload_ParseErr(t *testing.T) {
	// Any type other than the Workload type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}
	res := createWorkloadResource(t, data)

	err := MutateWorkload(res)
	require.Error(t, err)
	require.True(t, errors.As(err, &ErrDataParse{}))
}

func TestMutateWorkload_NotModified(t *testing.T) {
	data := validWorkload()
	data.Identity = "custom-identity"

	res := createWorkloadResource(t, data)

	require.NoError(t, MutateWorkload(res))

	var newData pbcatalog.Workload
	require.NoError(t, res.Data.UnmarshalTo(&newData))

	require.Equal(t, "custom-identity", newData.Identity)
}
