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

func createDNSPolicyResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: DNSPolicyType,
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

func TestValidateDNSPolicy_Ok(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Passing: 3,
			Warning: 1234,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.NoError(t, err)
}

func TestValidateDNSPolicy_ParseError(t *testing.T) {
	// Any type other than the DNSPolicy type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	require.True(t, errors.As(err, &ErrDataParse{}))
}

func TestValidateDNSPolicy_MissingWeights(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "weights",
		Wrapped: errMissing,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_InvalidPassingWeight(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Passing: 1000000,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "passing",
		Wrapped: errDNSWeightOutOfRange,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, "weights", actual.Name)
	err = actual.Unwrap()
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_InvalidWarningWeight(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		Weights: &pbcatalog.Weights{
			Warning: 1000000,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "warning",
		Wrapped: errDNSWeightOutOfRange,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, "weights", actual.Name)
	err = actual.Unwrap()
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_EmptySelector(t *testing.T) {
	data := &pbcatalog.DNSPolicy{
		Weights: &pbcatalog.Weights{
			Passing: 10,
			Warning: 3,
		},
	}

	res := createDNSPolicyResource(t, data)

	err := ValidateDNSPolicy(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "workloads",
		Wrapped: errEmpty,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateDNSPolicy_SelectorEmptyName(t *testing.T) {
	genData := func() *pbcatalog.DNSPolicy {
		return &pbcatalog.DNSPolicy{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"foo",
					"bar",
					"baz",
				},
			},
			Weights: &pbcatalog.Weights{
				Passing: 10,
				Warning: 3,
			},
		}
	}

	for i := 0; i < 3; i++ {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			data := genData()
			data.Workloads.Names[i] = ""

			res := createDNSPolicyResource(t, data)

			err := ValidateDNSPolicy(res)
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
