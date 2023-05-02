// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"strconv"
	"testing"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

func createServiceResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: ServiceType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "test-policy",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateService_Ok(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Names:    []string{"foo", "bar"},
			Prefixes: []string{"abc-"},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort:  "http-internal",
				VirtualPort: 42,
				Protocol:    pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
		VirtualIps: []string{"198.18.0.1"},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.NoError(t, err)
}

func TestValidateService_ParseError(t *testing.T) {
	// Any type other than the Service type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	require.True(t, errors.As(err, &ErrDataParse{}))
}

func TestValidateService_EmptySelector(t *testing.T) {
	data := &pbcatalog.Service{
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "http-internal",
				Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.NoError(t, err)
}

func TestValidateService_SelectorEmptyName(t *testing.T) {
	genData := func() *pbcatalog.Service {
		return &pbcatalog.Service{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"foo",
					"bar",
					"baz",
				},
			},
			Ports: []*pbcatalog.ServicePort{
				{
					TargetPort: "http-internal",
					Protocol:   pbcatalog.Protocol_PROTOCOL_HTTP,
				},
			},
		}
	}

	for i := 0; i < 3; i++ {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			data := genData()
			data.Workloads.Names[i] = ""

			res := createServiceResource(t, data)

			err := ValidateService(res)
			expected := ErrInvalidListElement{
				Name:    "names",
				Index:   i,
				Wrapped: errEmpty,
			}

			var actual ErrInvalidListElement
			require.True(t, errors.As(err, &actual))
			require.Equal(t, expected, actual)
		})
	}
}

func TestValidateService_InvalidTargetPort(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "",
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, "target_port", actual.Name)
	require.Equal(t, errEmpty, actual.Wrapped)
}

func TestValidateService_VirtualPortReused(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				VirtualPort: 42,
				TargetPort:  "foo",
			},
			{
				VirtualPort: 42,
				TargetPort:  "bar",
			},
		},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	var actual ErrVirtualPortReused
	require.True(t, errors.As(err, &actual))
	require.EqualValues(t, 0, actual.Index)
	require.EqualValues(t, 42, actual.Value)
}

func TestValidateService_InvalidVIP(t *testing.T) {
	data := &pbcatalog.Service{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Ports: []*pbcatalog.ServicePort{
			{
				TargetPort: "foo",
			},
		},
		VirtualIps: []string{"foo"},
	}

	res := createServiceResource(t, data)

	err := ValidateService(res)
	require.Error(t, err)
	require.True(t, errors.Is(err, errNotIPAddress))
}
