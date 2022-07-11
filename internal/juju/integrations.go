package juju

import (
	"time"

	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/rpc/params"
)

type integrationsClient struct {
	ConnectionFactory
}

type CreateIntegrationInput struct {
	ModelUUID string
	Endpoints []string
}

type CreateIntegrationResponse struct {
	Endpoints map[string]params.CharmRelation
}

type DestroyIntegrationInput struct {
	ModelUUID string
	Endpoints []string
}

func newIntegrationsClient(cf ConnectionFactory) *integrationsClient {
	return &integrationsClient{
		ConnectionFactory: cf,
	}
}

func (c integrationsClient) CreateIntegration(input *CreateIntegrationInput) (*CreateIntegrationResponse, error) {
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

	resp := CreateIntegrationResponse{
		Endpoints: response.Endpoints,
	}

	return &resp, nil
}

func (c integrationsClient) DestroyIntegration(input *DestroyIntegrationInput) error {
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
