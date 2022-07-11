package juju

import (
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
