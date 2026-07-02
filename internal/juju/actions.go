// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"context"
	"errors"
	"fmt"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/action"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/names/v5"
)

// EnqueueActionArgs holds the arguments to enqueue an action on a unit.
type EnqueueActionArgs struct {
	// ModelUUID is the UUID of the model where the action will be run.
	ModelUUID string
	// Receiver is the unit tag (e.g. "unit-ubuntu-0") or the unit name
	// (e.g. "ubuntu/0") that will receive the action. If the string is
	// a unit name it will be converted to a unit tag.
	Receiver string
	// Name is the name of the action to run.
	Name string
	// Parameters are the arguments to pass to the action.
	Parameters map[string]interface{}
}

// ActionResultArgs holds the arguments to read the result of an action.
type ActionResultArgs struct {
	// ModelUUID is the UUID of the model where the action was run.
	ModelUUID string
	// ActionID is the ID of the action to read.
	ActionID string
}

// ActionClient defines the interface for the actions client.
type ActionClient interface {
	// EnqueueAction enqueues an action to be run on the specified receiver.
	EnqueueAction(ctx context.Context, args EnqueueActionArgs) (string, error)
	// ActionResult reads the result of an action.
	ActionResult(ctx context.Context, args ActionResultArgs) (action.ActionResult, error)
}

// ResolveLeaderUnitArgs holds the arguments to resolve the leader unit of an
// application.
type ResolveLeaderUnitArgs struct {
	// ModelUUID is the UUID of the model.
	ModelUUID string
	// ApplicationName is the name of the application.
	ApplicationName string
}

// actionsClient is a client for running and reading Juju actions.
type actionsClient struct {
	SharedClient

	// getActionsAPIClient returns a new action API client for the given
	// connection. This is a field so it can be overridden in tests.
	getActionsAPIClient func(connection api.Connection) *action.Client
	// getClientAPIClient returns a new client API client for the given
	// connection. This is a field so it can be overridden in tests.
	getClientAPIClient func(connection api.Connection) ClientAPIClient
}

// newActionsClient creates a new actions client.
func newActionsClient(sc SharedClient) *actionsClient {
	return &actionsClient{
		SharedClient: sc,
		getActionsAPIClient: func(connection api.Connection) *action.Client {
			return action.NewClient(connection)
		},
		getClientAPIClient: func(connection api.Connection) ClientAPIClient {
			return apiclient.NewClient(connection, sc.JujuLogger())
		},
	}
}

// EnqueueAction enqueues an action to be run on the specified receiver.
// It returns the ID of the enqueued action.
func (c *actionsClient) EnqueueAction(ctx context.Context, args EnqueueActionArgs) (string, error) {
	conn, err := c.GetConnection(ctx, &args.ModelUUID)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	actionsAPIClient := c.getActionsAPIClient(conn)

	receiver := args.Receiver
	// If the receiver is not already a unit tag, try to parse it as a
	// unit name and convert it to a unit tag.
	if !isUnitTag(receiver) {
		unitTag, err := parseUnitTag(receiver)
		if err != nil {
			return "", fmt.Errorf("invalid receiver %q: %w", receiver, err)
		}
		receiver = unitTag.String()
	}

	enqueuedActions, err := actionsAPIClient.EnqueueOperation(ctx, []action.Action{{
		Receiver:   receiver,
		Name:       args.Name,
		Parameters: args.Parameters,
	}})
	if err != nil {
		return "", err
	}
	if len(enqueuedActions.Actions) != 1 {
		return "", fmt.Errorf("expected exactly one enqueued action, got %d", len(enqueuedActions.Actions))
	}
	if enqueuedActions.Actions[0].Error != nil {
		return "", enqueuedActions.Actions[0].Error
	}
	if enqueuedActions.Actions[0].Action == nil {
		return "", errors.New("enqueued action is nil")
	}
	return enqueuedActions.Actions[0].Action.ID, nil
}

// ActionResult reads the result of an action by its ID.
func (c *actionsClient) ActionResult(ctx context.Context, args ActionResultArgs) (action.ActionResult, error) {
	conn, err := c.GetConnection(ctx, &args.ModelUUID)
	if err != nil {
		return action.ActionResult{}, err
	}
	defer func() { _ = conn.Close() }()

	actionsAPIClient := c.getActionsAPIClient(conn)

	results, err := actionsAPIClient.Actions(ctx, []string{args.ActionID})
	if err != nil {
		return action.ActionResult{}, err
	}
	if len(results) != 1 {
		return action.ActionResult{}, fmt.Errorf("expected exactly one action result, got %d", len(results))
	}
	return results[0], nil
}

// isUnitTag returns true if the string is a valid unit tag.
func isUnitTag(s string) bool {
	_, err := names.ParseUnitTag(s)
	return err == nil
}

// parseUnitTag parses a unit name (e.g. "ubuntu/0") or a unit tag string
// and returns the corresponding names.UnitTag.
func parseUnitTag(s string) (names.UnitTag, error) {
	// Try to parse as a tag first.
	if tag, err := names.ParseUnitTag(s); err == nil {
		return tag, nil
	}
	// Otherwise, try to parse as a unit name.
	if !names.IsValidUnit(s) {
		return names.UnitTag{}, fmt.Errorf("%q is not a valid unit name", s)
	}
	return names.NewUnitTag(s), nil
}

// ResolveLeaderUnit returns the name of the leader unit for the given
// application. It is used when no specific unit is provided to run an
// action on.
func (c *actionsClient) ResolveLeaderUnit(ctx context.Context, args ResolveLeaderUnitArgs) (string, error) {
	conn, err := c.GetConnection(ctx, &args.ModelUUID)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	clientAPIClient := c.getClientAPIClient(conn)
	status, err := clientAPIClient.Status(ctx, nil)
	if err != nil {
		return "", err
	}

	appStatus, ok := status.Applications[args.ApplicationName]
	if !ok {
		return "", NewApplicationNotFoundError(args.ApplicationName)
	}

	// Find the leader unit. If no leader is found (e.g. for CAAS
	// applications), fall back to the first unit.
	for unitName, unit := range appStatus.Units {
		if unit.Leader {
			return unitName, nil
		}
	}

	for unitName := range appStatus.Units {
		return unitName, nil
	}

	return "", NewRetryReadError(fmt.Sprintf("no units found for application %q", args.ApplicationName))
}
