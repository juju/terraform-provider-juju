// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"

	"github.com/juju/juju/api/client/highavailability"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/core/constraints"
)

// EnableHAInput contains the input for enabling high availability on a controller.
type EnableHAInput struct {
	// ConnInfo holds the connection details for the target controller.
	ConnInfo ControllerConnectionInformation
	// Constraints is an optional constraint string for newly provisioned
	// controller units (e.g. "mem=8G cores=4").
	Constraints string
	// Units is the desired number of controller units. Must be odd and >= 3.
	Units int
	// To is an optional list of placement directives for the new controller
	// units (e.g. ["lxd:0", "lxd:1"]). When empty, Juju selects placement
	// automatically.
	To []string
}

// EnableHAClient handles high-availability operations against a Juju controller.
// It creates its own API connection using the connection details provided per
// invocation, keeping the client stateless and safe for concurrent use.
type EnableHAClient struct{}

// NewEnableHAClient returns a new EnableHAClient.
func NewEnableHAClient() *EnableHAClient {
	return &EnableHAClient{}
}

// EnableHA enables high availability on the controller described by input.ConnInfo.
//
// EnableHA is idempotent when called with the same number of units.
// Decreasing the number of units is not supported by Juju and will return an
// error from the API.
// An odd number of units or less than 3 units will also result in an error from the API.
func (c *EnableHAClient) EnableHA(ctx context.Context, input EnableHAInput) error {
	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: input.ConnInfo.Addresses,
		CACert:              input.ConnInfo.CACert,
		Username:            input.ConnInfo.Username,
		Password:            input.ConnInfo.Password,
	})
	if err != nil {
		return fmt.Errorf("failed to create connector: %w", err)
	}

	conn, err := connr.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to controller: %w", err)
	}
	defer conn.Close()

	haClient := highavailability.NewClient(conn)
	defer haClient.Close()

	var cons constraints.Value
	if input.Constraints != "" {
		cons, err = constraints.Parse(input.Constraints)
		if err != nil {
			return fmt.Errorf("failed to parse constraints %q: %w", input.Constraints, err)
		}
	}

	if _, err = haClient.EnableHA(input.Units, cons, input.To); err != nil {
		return fmt.Errorf("failed to enable HA: %w", err)
	}

	return nil
}
