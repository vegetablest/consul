package ctltest

import (
	"context"
	"testing"

	"github.com/hashicorp/consul/proto-public/pbresource"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/anypb"
)

type resourceBuilder struct {
	resource *pbresource.Resource
}

func Resource(rtype *pbresource.Type, name string) *resourceBuilder {
	return &resourceBuilder{
		resource: &pbresource.Resource{
			Id: &pbresource.ID{
				Type: &pbresource.Type{
					Group:        rtype.Group,
					GroupVersion: rtype.GroupVersion,
					Kind:         rtype.Kind,
				},
				Tenancy: &pbresource.Tenancy{
					Partition: "default",
					Namespace: "default",
					PeerName:  "local",
				},
				Name: name,
			},
		},
	}
}

func (b *resourceBuilder) WithData(t *testing.T, data protoreflect.ProtoMessage) *resourceBuilder {
	anyData, err := anypb.New(data)
	require.NoError(t, err)
	b.resource.Data = anyData
	return b
}

func (b *resourceBuilder) WithOwner(id *pbresource.ID) *resourceBuilder {
	b.resource.Owner = id
	return b
}

func (b *resourceBuilder) Build() *pbresource.Resource {
	return b.resource
}

func (b *resourceBuilder) Write(t *testing.T, client pbresource.ResourceServiceClient) *pbresource.Resource {
	res := b.Build()

	rsp, err := client.Write(context.Background(), &pbresource.WriteRequest{
		Resource: res,
	})

	require.NoError(t, err)
	return rsp.Resource
}
