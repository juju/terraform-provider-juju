// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// In comments and in code we refer to integrations which are known in juju 2.x as relations.
// calls to the upstream juju client currently reference relations
package juju

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/juju/errors"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/rpc/params"
)

const (
	// IntegrationQueryTick defines the time to wait between ticks
	// when querying the API
	IntegrationApiTickWait = time.Second * 5
	// IntegrationAppAvailableTimeout indicates the time to wait
	// for applications to be available before integrating them
	IntegrationAppAvailableTimeout = time.Second * 60
)

// IntegrationNotFoundError is returned when an integration cannot be found.
var IntegrationNotFoundError = errors.ConstError("integration-not-found")

// NewIntegrationNotFoundError creates a new error indicating that no integration was found
// for the given model UUID.
func NewIntegrationNotFoundError(modelUUID string) error {
	return errors.WithType(errors.Errorf("no integration found for model %s", modelUUID), IntegrationNotFoundError)
}

type integrationsClient struct {
	SharedClient
}

type Application struct {
	Name     string
	Endpoint string
	Role     string
	OfferURL *string
}

type Offer struct {
	OfferURL string
}

type IntegrationInput struct {
	ModelUUID string
	Apps      []string
	Endpoints []string
	ViaCIDRs  string
}

type CreateIntegrationResponse struct {
	Applications []Application
}

type ReadIntegrationResponse struct {
	Applications []Application
}

type UpdateIntegrationResponse struct {
	Applications []Application
}

type UpdateIntegrationInput struct {
	ModelUUID    string
	Endpoints    []string
	OldEndpoints []string
	ViaCIDRs     string
}

func newIntegrationsClient(sc SharedClient) *integrationsClient {
	return &integrationsClient{
		SharedClient: sc,
	}
}

func (c integrationsClient) CreateIntegration(input *IntegrationInput) (*CreateIntegrationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := apiapplication.NewClient(conn)

	// wait for the apps to be available
	ctx, cancel := context.WithTimeout(context.Background(), IntegrationAppAvailableTimeout)
	defer cancel()

	err = WaitForAppsAvailable(ctx, client, input.Apps, IntegrationApiTickWait)
	if err != nil {
		return nil, errors.New("the applications were not available to be integrated")
	}

	listViaCIDRs := splitCommaDelimitedList(input.ViaCIDRs)
	response, err := client.AddRelation(
		input.Endpoints,
		listViaCIDRs,
	)
	if err != nil {
		return nil, err
	}

	// integration is created - fetch the status in order to validate
	status, err := c.ModelStatus(input.ModelUUID, conn)
	if err != nil {
		return nil, err
	}

	applications, err := parseApplications(status.RemoteApplications, response.Endpoints)
	if err != nil {
		return nil, err
	}
	c.Debugf("related apps", map[string]any{"apps": applications})

	return &CreateIntegrationResponse{
		Applications: applications,
	}, nil
}

func (c integrationsClient) ReadIntegration(input *IntegrationInput) (*ReadIntegrationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	modelUUID, ok := conn.ModelTag()
	if !ok {
		return nil, errors.Errorf("Unable to get model uuid for %q", input.ModelUUID)
	}
	status, err := c.ModelStatus(input.ModelUUID, conn)
	if err != nil {
		return nil, err
	}

	integrations := status.Relations
	var integration params.RelationStatus
	if len(integrations) == 0 {
		return nil, NewIntegrationNotFoundError(modelUUID.Id())
	}

	apps := make([][]string, 0, len(input.Endpoints))
	for _, v := range input.Endpoints {
		app := strings.Split(v, ":")
		apps = append(apps, []string{
			app[0],
			app[1],
		})
	}

	// the key is built assuming that the ID is "<provider>:<endpoint> <requirer>:<endpoint>"
	// the integrations that come back from status have the key formatted as "<requirer>:<endpoint> <provider>:<endpoint>"
	key := fmt.Sprintf("%v:%v %v:%v", apps[1][0], apps[1][1], apps[0][0], apps[0][1])

	for _, v := range integrations {
		if v.Key == key {
			integration = v
			break
		}
	}

	if integration.Id == 0 && integration.Key == "" {
		keyReversed := fmt.Sprintf("%v:%v %v:%v", apps[1][0], apps[1][1], apps[0][0], apps[0][1])
		for _, v := range integrations {
			if v.Key == keyReversed {
				return nil, fmt.Errorf("check the endpoint order in your ID")
			}
		}
	}

	if integration.Id == 0 && integration.Key == "" {
		return nil, NewIntegrationNotFoundError(modelUUID.Id())
	}

	applications, err := parseApplications(status.RemoteApplications, integration.Endpoints)
	if err != nil {
		return nil, err
	}

	return &ReadIntegrationResponse{
		Applications: applications,
	}, nil
}

func (c integrationsClient) DestroyIntegration(input *IntegrationInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := apiapplication.NewClient(conn)

	err = client.DestroyRelation(
		nil,
		nil,
		input.Endpoints...,
	)
	if err != nil {
		return err
	}

	return nil
}

// This function takes remote applications and endpoint status and combines them into a more usable format to return to the provider
func parseApplications(remoteApplications map[string]params.RemoteApplicationStatus, src interface{}) ([]Application, error) {
	applications := make([]Application, 0, 2)

	switch endpoints := src.(type) {
	case []params.EndpointStatus:
		for index, endpoint := range endpoints {
			if remote, exists := remoteApplications[endpoint.ApplicationName]; exists {
				if remote.OfferURL != "" {
					url, err := removeOfferURLSource(remote.OfferURL)
					if err != nil {
						return nil, err
					}
					remote.OfferURL = url
				}

				a := Application{
					Name:     endpoint.ApplicationName,
					Endpoint: endpoint.Name,
					Role:     endpoint.Role,
					OfferURL: &remote.OfferURL,
				}
				applications = append(applications, a)

				endpoints[index] = endpoints[len(endpoints)-1]
				endpoints = endpoints[:len(endpoints)-1]
			}
		}
		for _, endpoint := range endpoints {
			a := Application{
				Name:     endpoint.ApplicationName,
				Endpoint: endpoint.Name,
				Role:     endpoint.Role,
				OfferURL: nil,
			}
			applications = append(applications, a)
		}
	case map[string]params.CharmRelation:
		if len(remoteApplications) != 0 {
			for index, endpoint := range endpoints {
				if remote, exists := remoteApplications[index]; exists {
					a := Application{
						Name:     index,
						Endpoint: endpoint.Name,
						Role:     endpoint.Role,
						OfferURL: &remote.OfferURL,
					}
					applications = append(applications, a)

					delete(endpoints, index)
				}
			}
		}
		for index, endpoint := range endpoints {
			a := Application{
				Name:     index,
				Endpoint: endpoint.Name,
				Role:     endpoint.Role,
				OfferURL: nil,
			}
			applications = append(applications, a)
		}
	}

	return applications, nil
}
