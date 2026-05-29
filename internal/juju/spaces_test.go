// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/juju/juju/api"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestSpacesClientCreateSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSharedClient := NewMockSharedClient(ctrl)
	mockConnection := NewMockConnection(ctrl)
	mockSpacesAPIClient := NewMockSpacesAPIClient(ctrl)

	mockSharedClient.EXPECT().GetConnection(gomock.Any(), gomock.Any()).Return(mockConnection, nil)
	mockConnection.EXPECT().Close().Return(nil)
	mockSpacesAPIClient.EXPECT().CreateSpace(gomock.Any(), "public", nil, true).Return(nil)

	client := &spacesClient{
		SharedClient: mockSharedClient,
		getSpacesAPIClient: func(api.Connection) SpacesAPIClient {
			return mockSpacesAPIClient
		},
	}

	err := client.CreateSpace(t.Context(), &CreateSpaceInput{ModelUUID: "model-uuid", Name: "public"})
	require.NoError(t, err)
}

func TestSpacesClientMoveSubnetToSpaceRequiresCIDR(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := &spacesClient{SharedClient: NewMockSharedClient(ctrl)}

	err := client.MoveSubnetToSpace(t.Context(), &MoveSubnetToSpaceInput{ModelUUID: "model-uuid", SpaceName: "public"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cidr")
}

func TestSpacesClientMoveSubnetToSpace(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockSharedClient := NewMockSharedClient(ctrl)
	mockConnection := NewMockConnection(ctrl)
	mockSpacesAPIClient := NewMockSpacesAPIClient(ctrl)
	mockSubnetsAPIClient := NewMockSubnetsAPIClient(ctrl)

	mockSharedClient.EXPECT().GetConnection(gomock.Any(), gomock.Any()).Return(mockConnection, nil)
	mockConnection.EXPECT().Close().Return(nil)
	mockSubnetsAPIClient.EXPECT().SubnetsByCIDR(gomock.Any(), []string{"10.0.0.0/24"}).Return([]params.SubnetsResult{{
		Subnets: []params.SubnetV2{{
			ID: "42",
			Subnet: params.Subnet{
				CIDR: "10.0.0.0/24",
			},
		}},
	}}, nil)
	mockSpacesAPIClient.EXPECT().MoveSubnets(
		gomock.Any(),
		names.NewSpaceTag("space-a"),
		[]names.SubnetTag{names.NewSubnetTag("42")},
		false,
	).Return(params.MoveSubnetsResult{}, nil)

	client := &spacesClient{
		SharedClient: mockSharedClient,
		getSpacesAPIClient: func(api.Connection) SpacesAPIClient {
			return mockSpacesAPIClient
		},
		getSubnetsAPIClient: func(api.Connection) SubnetsAPIClient {
			return mockSubnetsAPIClient
		},
	}

	err := client.MoveSubnetToSpace(t.Context(), &MoveSubnetToSpaceInput{
		ModelUUID: "model-uuid",
		SpaceName: "space-a",
		CIDR:      "10.0.0.0/24",
	})
	require.NoError(t, err)
}
