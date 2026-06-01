// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"context"

	"github.com/juju/errors"
	"github.com/juju/juju/api"
	apispaces "github.com/juju/juju/api/client/spaces"
	apisubnets "github.com/juju/juju/api/client/subnets"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
)

type spacesClient struct {
	SharedClient

	getSpacesAPIClient  func(api.Connection) SpacesAPIClient
	getSubnetsAPIClient func(api.Connection) SubnetsAPIClient
}

// ReadSpaceOutput is the provider-facing representation of a Juju space.
type ReadSpaceOutput struct {
	ID      string
	Name    string
	Subnets []params.Subnet
}

// ListSpacesInput contains the parameters for listing spaces.
type ListSpacesInput struct {
	ModelUUID string
}

// ListSpacesOutput is the provider-facing representation of a listed Juju space.
type ListSpacesOutput struct {
	ID      string
	Name    string
	Subnets []params.Subnet
}

// CreateSpaceInput contains the parameters for creating a space.
type CreateSpaceInput struct {
	ModelUUID string
	Name      string
}

// ReadSpaceInput contains the parameters for reading a space.
type ReadSpaceInput struct {
	ModelUUID string
	Name      string
}

// DeleteSpaceInput contains the parameters for deleting a space.
type DeleteSpaceInput struct {
	ModelUUID string
	Name      string
}

// MoveSubnetToSpaceInput contains the parameters for moving one subnet into a space.
type MoveSubnetToSpaceInput struct {
	ModelUUID string
	SpaceName string
	CIDR      string
}

func newSpacesClient(sc SharedClient) *spacesClient {
	return &spacesClient{
		SharedClient: sc,
		getSpacesAPIClient: func(conn api.Connection) SpacesAPIClient {
			return apispaces.NewAPI(conn)
		},
		getSubnetsAPIClient: func(conn api.Connection) SubnetsAPIClient {
			return apisubnets.NewAPI(conn)
		},
	}
}

// CreateSpace creates a space without assigning any subnets at creation time.
func (c *spacesClient) CreateSpace(ctx context.Context, input *CreateSpaceInput) error {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	spaceClient := c.getSpacesAPIClient(conn)
	// The public argument isn't actually implemented server side, and defaults to true
	// in the client - so we do too.
	if err := spaceClient.CreateSpace(ctx, input.Name, nil, true); err != nil {
		return typedError(errors.Annotate(err, "creating space"))
	}
	return nil
}

// ReadSpace reads a space by name.
func (c *spacesClient) ReadSpace(ctx context.Context, input *ReadSpaceInput) (*ReadSpaceOutput, error) {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	spaceClient := c.getSpacesAPIClient(conn)
	result, err := spaceClient.ShowSpace(ctx, input.Name)
	if err != nil {
		return nil, typedError(errors.Annotate(err, "reading space"))
	}

	return &ReadSpaceOutput{
		ID:      result.Space.Id,
		Name:    result.Space.Name,
		Subnets: result.Space.Subnets,
	}, nil
}

// ListSpaces lists all spaces in a model.
func (c *spacesClient) ListSpaces(ctx context.Context, input *ListSpacesInput) ([]ListSpacesOutput, error) {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	spaceClient := c.getSpacesAPIClient(conn)
	spaces, err := spaceClient.ListSpaces(ctx)
	if err != nil {
		return nil, typedError(errors.Annotate(err, "listing spaces"))
	}

	result := make([]ListSpacesOutput, len(spaces))
	for i, space := range spaces {
		result[i] = ListSpacesOutput{
			ID:      space.Id,
			Name:    space.Name,
			Subnets: space.Subnets,
		}
	}

	return result, nil
}

// DeleteSpace removes a space.
func (c *spacesClient) DeleteSpace(ctx context.Context, input *DeleteSpaceInput) error {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	spaceClient := c.getSpacesAPIClient(conn)
	_, err = spaceClient.RemoveSpace(ctx, input.Name, false, false)
	if err != nil {
		return typedError(errors.Annotate(err, "deleting space"))
	}

	return nil
}

// MoveSubnetToSpace moves one CIDR into the requested space.
func (c *spacesClient) MoveSubnetToSpace(ctx context.Context, input *MoveSubnetToSpaceInput) error {
	if input.CIDR == "" {
		return errors.NotValidf("cidr")
	}

	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	subnetsClient := c.getSubnetsAPIClient(conn)
	subnetResults, err := subnetsClient.SubnetsByCIDR(ctx, []string{input.CIDR})
	if err != nil {
		return typedError(errors.Annotate(err, "looking up subnet IDs by CIDR"))
	}

	// Find the CIDR in the returned list, we expect 1 result
	subnetID, err := findSubnetIDByCIDR(subnetResults, input.CIDR)
	if err != nil {
		return err
	}

	subnetTags := []names.SubnetTag{names.NewSubnetTag(subnetID)}

	spaceClient := c.getSpacesAPIClient(conn)
	_, err = spaceClient.MoveSubnets(ctx, names.NewSpaceTag(input.SpaceName), subnetTags, false)
	if err != nil {
		return typedError(errors.Annotate(err, "moving subnets"))
	}
	return nil
}

func findSubnetIDByCIDR(subnetResults []params.SubnetsResult, cidr string) (string, error) {
	var subnetID string
	for _, result := range subnetResults {
		for _, subnet := range result.Subnets {
			if subnet.CIDR == cidr {
				subnetID = subnet.ID
				break
			}
		}
		if subnetID != "" {
			break
		}
	}
	if subnetID == "" {
		return "", errors.NotFoundf("subnet for CIDR %q", cidr)
	}
	return subnetID, nil
}
