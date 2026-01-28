// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"

	"github.com/juju/juju/api"
	"github.com/juju/names/v5"
)

type infoParameters struct {
	Tag     string `json:"tag"`
	Channel string `json:"channel,omitempty"`
}

type charmHubEntityInfoResult struct {
	Result infoResponse          `json:"result"`
	Errors charmhubErrorResponse `json:"errors"`
}

type charmhubErrorResponse struct {
	Error charmhubError `json:"error-list"`
}

type charmhubError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type infoResponse struct {
	ID    string         `json:"id"`
	Name  string         `json:"name"`
	Charm *charmHubCharm `json:"charm,omitempty"`
}

type charmHubCharm struct {
	Subordinate bool `json:"subordinate"`
}

// IsSubordinateCharmParameters holds the parameters for the IsSubordinateCharm method.
type IsSubordinateCharmParameters struct {
	Name    string
	Channel string
}

type charmsClient struct {
	Connection api.Connection
}

// newCharmsClient creates a new charmsClient that works with Juju 2.9 controller to fetch
// information on whether a charm is a subordinate charm.
func newCharmsClient(conn api.Connection) *charmsClient {
	return &charmsClient{
		Connection: conn,
	}
}

// IsSubordinateCharm checks if the given charm is a subordinate charm.
func (c *charmsClient) IsSubordinateCharm(input IsSubordinateCharmParameters) (bool, error) {
	args := infoParameters{
		Tag:     names.NewApplicationTag(input.Name).String(),
		Channel: input.Channel,
	}
	result := charmHubEntityInfoResult{}

	err := c.Connection.APICall("CharmHub", 1, "", "Info", args, &result)
	if err != nil {
		return false, err
	}

	if result.Errors.Error.Message != "" {
		return false,
			fmt.Errorf("failed to fetch charm info: got error with code %s and message %s",
				result.Errors.Error.Code,
				result.Errors.Error.Message,
			)
	}

	return result.Result.Charm.Subordinate, nil
}
