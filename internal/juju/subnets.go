// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"context"
	"strings"

	"github.com/juju/errors"
	"github.com/juju/juju/api"
	apisubnets "github.com/juju/juju/api/client/subnets"
	"github.com/juju/juju/core/life"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
)

const spaceTagPrefix = "space-"

type subnetsClient struct {
	SharedClient

	getSubnetsAPIClient func(api.Connection) SubnetsAPIClient
}

// SubnetInfo is the provider-facing representation of a Juju subnet.
type SubnetInfo struct {
	ID                string
	CIDR              string
	ProviderID        string
	ProviderNetworkID string
	ProviderSpaceID   string
	VLANTag           int
	Life              life.Value
	SpaceName         string
	Zones             []string
}

// ListSubnetsInput contains filter values for listing subnets.
type ListSubnetsInput struct {
	ModelUUID string
	SpaceName string
	Zone      string
}

// ReadSubnetInput contains the parameters for reading a subnet by CIDR.
type ReadSubnetInput struct {
	ModelUUID string
	CIDR      string
}

func newSubnetsClient(sc SharedClient) *subnetsClient {
	return &subnetsClient{
		SharedClient: sc,
		getSubnetsAPIClient: func(conn api.Connection) SubnetsAPIClient {
			return apisubnets.NewAPI(conn)
		},
	}
}

// ListSubnets fetches all model subnets matching optional space/zone filters.
func (c *subnetsClient) ListSubnets(ctx context.Context, input *ListSubnetsInput) ([]SubnetInfo, error) {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	var spaceTag *names.SpaceTag
	if input.SpaceName != "" {
		tag := names.NewSpaceTag(input.SpaceName)
		spaceTag = &tag
	}

	subnetsClient := c.getSubnetsAPIClient(conn)
	subnets, err := subnetsClient.ListSubnets(ctx, spaceTag, input.Zone)
	if err != nil {
		return nil, typedError(errors.Annotate(err, "listing subnets"))
	}

	result := make([]SubnetInfo, len(subnets))
	for i, subnet := range subnets {
		result[i] = subnetFromParamsSubnet(subnet)
	}

	return result, nil
}

// ReadSubnet fetches a single subnet by CIDR.
func (c *subnetsClient) ReadSubnet(ctx context.Context, input *ReadSubnetInput) (*SubnetInfo, error) {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	subnetsClient := c.getSubnetsAPIClient(conn)
	results, err := subnetsClient.SubnetsByCIDR(ctx, []string{input.CIDR})
	if err != nil {
		return nil, typedError(errors.Annotate(err, "reading subnet"))
	}
	if len(results) == 0 {
		return nil, errors.NotFoundf("subnet %q", input.CIDR)
	}

	for _, subnet := range results[0].Subnets {
		if subnet.CIDR != input.CIDR {
			continue
		}
		info := subnetFromParamsSubnetV2(subnet)
		return &info, nil
	}
	return nil, errors.NotFoundf("subnet %q", input.CIDR)
}

func subnetFromParamsSubnet(subnet params.Subnet) SubnetInfo {
	return SubnetInfo{
		CIDR:              subnet.CIDR,
		ProviderID:        subnet.ProviderId,
		ProviderNetworkID: subnet.ProviderNetworkId,
		ProviderSpaceID:   subnet.ProviderSpaceId,
		VLANTag:           subnet.VLANTag,
		Life:              subnet.Life,
		SpaceName:         spaceNameFromTag(subnet.SpaceTag),
		Zones:             subnet.Zones,
	}
}

func subnetFromParamsSubnetV2(subnet params.SubnetV2) SubnetInfo {
	result := subnetFromParamsSubnet(subnet.Subnet)
	result.ID = subnet.ID
	return result
}

func spaceNameFromTag(spaceTag string) string {
	if strings.HasPrefix(spaceTag, spaceTagPrefix) {
		return strings.TrimPrefix(spaceTag, spaceTagPrefix)
	}
	return spaceTag
}
