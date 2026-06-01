// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/juju/juju/api"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSubnetsClientListSubnets(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSharedClient := NewMockSharedClient(ctrl)
	mockConnection := NewMockConnection(ctrl)
	mockSubnetsAPIClient := NewMockSubnetsAPIClient(ctrl)

	mockSharedClient.EXPECT().GetConnection(gomock.Any(), gomock.Any()).Return(mockConnection, nil)
	mockConnection.EXPECT().Close().Return(nil)
	mockSubnetsAPIClient.EXPECT().ListSubnets(
		gomock.Any(),
		gomock.AssignableToTypeOf((*names.SpaceTag)(nil)),
		"zone-1",
	).Return([]params.Subnet{{
		CIDR:            "10.0.0.0/24",
		SpaceTag:        "space-public",
		ProviderId:      "subnet-id",
		ProviderSpaceId: "space-id",
		Life:            life.Alive,
		Zones:           []string{"zone-1"},
	}}, nil)

	client := &subnetsClient{
		SharedClient: mockSharedClient,
		getSubnetsAPIClient: func(api.Connection) SubnetsAPIClient {
			return mockSubnetsAPIClient
		},
	}

	subnets, err := client.ListSubnets(t.Context(), &ListSubnetsInput{
		ModelUUID: "model-uuid",
		SpaceName: "public",
		Zone:      "zone-1",
	})
	require.NoError(t, err)
	require.Len(t, subnets, 1)
	require.Equal(t, "10.0.0.0/24", subnets[0].CIDR)
	require.Equal(t, "public", subnets[0].SpaceName)
}

func TestSubnetsClientReadSubnet(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSharedClient := NewMockSharedClient(ctrl)
	mockConnection := NewMockConnection(ctrl)
	mockSubnetsAPIClient := NewMockSubnetsAPIClient(ctrl)

	mockSharedClient.EXPECT().GetConnection(gomock.Any(), gomock.Any()).Return(mockConnection, nil)
	mockConnection.EXPECT().Close().Return(nil)
	mockSubnetsAPIClient.EXPECT().SubnetsByCIDR(gomock.Any(), []string{"10.0.0.0/24"}).Return([]params.SubnetsResult{{
		Subnets: []params.SubnetV2{{
			ID: "subnet-uuid",
			Subnet: params.Subnet{
				CIDR:            "10.0.0.0/24",
				SpaceTag:        "space-public",
				ProviderId:      "subnet-id",
				ProviderSpaceId: "space-id",
				Life:            life.Alive,
				Zones:           []string{"zone-1"},
			},
		}},
	}}, nil)

	client := &subnetsClient{
		SharedClient: mockSharedClient,
		getSubnetsAPIClient: func(api.Connection) SubnetsAPIClient {
			return mockSubnetsAPIClient
		},
	}

	subnet, err := client.ReadSubnet(t.Context(), &ReadSubnetInput{ModelUUID: "model-uuid", CIDR: "10.0.0.0/24"})
	require.NoError(t, err)
	require.Equal(t, "subnet-uuid", subnet.ID)
	require.Equal(t, "public", subnet.SpaceName)
}
