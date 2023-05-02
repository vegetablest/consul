// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package types

import (
	"errors"
	"strconv"
	"testing"
	"time"

	pbcatalog "github.com/hashicorp/consul/proto-public/pbcatalog/v1alpha1"
	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
)

func createHealthChecksResource(t *testing.T, data protoreflect.ProtoMessage) *pbresource.Resource {
	res := &pbresource.Resource{
		Id: &pbresource.ID{
			Type: HealthChecksType,
			Tenancy: &pbresource.Tenancy{
				Partition: "default",
				Namespace: "default",
				PeerName:  "local",
			},
			Name: "test-checks",
		},
	}

	var err error
	res.Data, err = anypb.New(data)
	require.NoError(t, err)
	return res
}

func TestValidateHealthChecks_Ok(t *testing.T) {
	data := &pbcatalog.HealthChecks{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		HealthChecks: []*pbcatalog.HealthCheck{
			{
				Name: "test-check",
				Definition: &pbcatalog.HealthCheck_Tcp{
					Tcp: &pbcatalog.TCPCheck{
						Address: "198.18.0.1",
					},
				},
				Interval: durationpb.New(30 * time.Second),
				Timeout:  durationpb.New(15 * time.Second),
			},
		},
	}

	res := createHealthChecksResource(t, data)

	err := ValidateHealthChecks(res)
	require.NoError(t, err)
}

func TestValidateHealthChecks_ParseError(t *testing.T) {
	// Any type other than the HealthChecks type would work
	// to cause the error we are expecting
	data := &pbcatalog.IP{Address: "198.18.0.1"}

	res := createHealthChecksResource(t, data)

	err := ValidateHealthChecks(res)
	require.Error(t, err)
	require.True(t, errors.As(err, &ErrDataParse{}))
}

func TestValidateHealthChecks_InvalidCheckName(t *testing.T) {
	genData := func(name string) *pbcatalog.HealthChecks {
		return &pbcatalog.HealthChecks{
			Workloads: &pbcatalog.WorkloadSelector{
				Prefixes: []string{""},
			},
			HealthChecks: []*pbcatalog.HealthCheck{
				{
					Name: name,
					Definition: &pbcatalog.HealthCheck_Tcp{
						Tcp: &pbcatalog.TCPCheck{
							Address: "198.18.0.1",
						},
					},
					Interval: durationpb.New(30 * time.Second),
					Timeout:  durationpb.New(15 * time.Second),
				},
			},
		}
	}

	type testCase struct {
		name        string
		err         bool
		expectedErr ErrInvalidField
	}

	// These checks are not exhaustive of the classes of names which
	// would be accepted or rejected. The tests for the isValidDNSLabel
	// function have more thorough testing. Here we just ensure that
	// we can get the errNotDNSLabel error to indicate that calling
	// that function returned false and was emitted by ValidateHealthChecks
	cases := map[string]testCase{
		"basic": {
			name: "foo-check",
		},
		"missing-name": {
			err: true,
			expectedErr: ErrInvalidField{
				Name:    "name",
				Wrapped: errEmpty,
			},
		},
		"invalid-dns-label": {
			name: "UPPERCASE",
			err:  true,
			expectedErr: ErrInvalidField{
				Name:    "name",
				Wrapped: errNotDNSLabel,
			},
		},
	}

	for name, tcase := range cases {
		t.Run(name, func(t *testing.T) {
			data := genData(tcase.name)
			err := ValidateHealthChecks(createHealthChecksResource(t, data))

			if tcase.err {
				require.Error(t, err)

			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateHealthChecks_MissingDefinition(t *testing.T) {
	data := &pbcatalog.HealthChecks{
		Workloads: &pbcatalog.WorkloadSelector{
			Prefixes: []string{""},
		},
		HealthChecks: []*pbcatalog.HealthCheck{
			{
				Name:     "test-check",
				Interval: durationpb.New(30 * time.Second),
				Timeout:  durationpb.New(15 * time.Second),
			},
		},
	}

	res := createHealthChecksResource(t, data)

	err := ValidateHealthChecks(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "definition",
		Wrapped: errMissing,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateHealthChecks_EmptySelector(t *testing.T) {
	data := &pbcatalog.HealthChecks{
		HealthChecks: []*pbcatalog.HealthCheck{
			{
				Name: "test-check",
				Definition: &pbcatalog.HealthCheck_Tcp{
					Tcp: &pbcatalog.TCPCheck{
						Address: "198.18.0.1",
					},
				},
				Interval: durationpb.New(30 * time.Second),
				Timeout:  durationpb.New(15 * time.Second),
			},
		},
	}

	res := createHealthChecksResource(t, data)

	err := ValidateHealthChecks(res)
	require.Error(t, err)
	expected := ErrInvalidField{
		Name:    "workloads",
		Wrapped: errEmpty,
	}
	var actual ErrInvalidField
	require.True(t, errors.As(err, &actual))
	require.Equal(t, expected, actual)
}

func TestValidateHealthChecks_SelectorEmptyName(t *testing.T) {
	genData := func() *pbcatalog.HealthChecks {
		return &pbcatalog.HealthChecks{
			Workloads: &pbcatalog.WorkloadSelector{
				Names: []string{
					"foo",
					"bar",
					"baz",
				},
			},
			HealthChecks: []*pbcatalog.HealthCheck{
				{
					Name: "test-check",
					Definition: &pbcatalog.HealthCheck_Tcp{
						Tcp: &pbcatalog.TCPCheck{
							Address: "198.18.0.1",
						},
					},
					Interval: durationpb.New(30 * time.Second),
					Timeout:  durationpb.New(15 * time.Second),
				},
			},
		}
	}

	for i := 0; i < 3; i++ {
		t.Run(strconv.FormatInt(int64(i), 10), func(t *testing.T) {
			data := genData()
			data.Workloads.Names[i] = ""

			res := createHealthChecksResource(t, data)

			err := ValidateHealthChecks(res)
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
