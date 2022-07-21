package juju

import (
	"fmt"
	"strings"
	"time"

	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/rpc/params"
)

type integrationsClient struct {
	ConnectionFactory
}

type IntegrationInput struct {
	ModelUUID string
	Endpoints []string
}

type UpdateIntegrationInput struct {
	ModelUUID    string
	ID           string
	Endpoints    []string
	OldEndpoints []string
}

type IntegrationResponse struct {
	Endpoints map[string]params.CharmRelation
}

type ReadIntegrationResponse struct {
	EndpointStatuses []params.EndpointStatus
}

func newIntegrationsClient(cf ConnectionFactory) *integrationsClient {
	return &integrationsClient{
		ConnectionFactory: cf,
	}
}

func (c integrationsClient) CreateIntegration(input *IntegrationInput) (*IntegrationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	client := apiapplication.NewClient(conn)
	defer client.Close()

	response, err := client.AddRelation(
		input.Endpoints,
		[]string(nil),
	)
	if err != nil {
		return nil, err
	}

	return &IntegrationResponse{Endpoints: response.Endpoints}, nil
}

func (c integrationsClient) ReadIntegration(input *IntegrationInput) (*ReadIntegrationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	apps := make([][]string, 0, len(input.Endpoints))
	for _, v := range input.Endpoints {
		app := strings.Split(v, ":")
		apps = append(apps, []string{
			app[0],
			app[1],
		})
	}

	client := apiapplication.NewClient(conn)
	defer client.Close()

	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}

	relations := status.Relations

	var relation params.RelationStatus

	if len(relations) == 0 {
		return nil, fmt.Errorf("no relations exist in specified model")
	}

	// the key is built assuming that the ID is "<provider>:<endpoint> <requirer>:<endpoint>"
	// the relations that come back from status have the key formatted as "<requirer>:<endpoint> <provider>:<endpoint>"
	key := fmt.Sprintf("%v:%v %v:%v", apps[1][0], apps[1][1], apps[0][0], apps[0][1])

	for _, v := range relations {
		if v.Key == key {
			relation = v
			break
		}
	}

	if relation.Id == 0 && relation.Key == "" {
		keyReversed := fmt.Sprintf("%v:%v %v:%v", apps[1][0], apps[1][1], apps[0][0], apps[0][1])
		for _, v := range relations {
			if v.Key == keyReversed {
				return nil, fmt.Errorf("check the endpoint order in your ID")
			}
		}
	}

	if relation.Id == 0 && relation.Key == "" {
		return nil, fmt.Errorf("relation not found in model")
	}

	return &ReadIntegrationResponse{EndpointStatuses: relation.Endpoints}, nil
}

func (c integrationsClient) UpdateIntegration(input *UpdateIntegrationInput) (*IntegrationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	client := apiapplication.NewClient(conn)
	defer client.Close()

	response, err := client.AddRelation(
		input.Endpoints,
		[]string(nil),
	)
	if err != nil {
		return nil, err
	}

	//TODO: check integration status

	var force bool = false
	var timeout time.Duration = 30 * time.Second
	err = client.DestroyRelation(
		&force,
		&timeout,
		input.OldEndpoints...,
	)
	if err != nil {
		return nil, err
	}

	//TODO: check deletion success and force?

	return &IntegrationResponse{Endpoints: response.Endpoints}, nil
}

func (c integrationsClient) DestroyIntegration(input *IntegrationInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	client := apiapplication.NewClient(conn)
	defer client.Close()

	var force bool = false
	var timeout time.Duration = 30 * time.Second

	err = client.DestroyRelation(
		&force,
		&timeout,
		input.Endpoints...,
	)
	if err != nil {
		return err
	}

	return nil
}
