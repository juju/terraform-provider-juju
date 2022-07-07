package juju

import (
	"fmt"
	"strings"

	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
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

	var endpoints []string = input.Endpoints

	for i := range endpoints {
		val, err := validateEndpoint(endpoints[i], client)
		if err != nil {
			return nil, err
		}
		endpoints[i] = val
	}

	response, err := client.AddRelation(
		endpoints,
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

func validateEndpoint(endpoint string, client *apiapplication.Client) (string, error) {
	var separator string = ":"
	spl := strings.Split(endpoint, separator)

	if spl[1] == "" {
		return getDefaultEndpoint(spl[0], client)
	}

	return endpoint, nil
}

func getDefaultEndpoint(applicationName string, client *apiapplication.Client) (string, error) {

	apps, err := client.ApplicationsInfo([]names.ApplicationTag{names.NewApplicationTag(applicationName)})
	if err != nil {
		return "", err
	}
	if len(apps) > 1 || len(apps) == 0 {
		return "", fmt.Errorf("unable to find single application called %s", applicationName)
	}
	endpointBindings := apps[0].Result.EndpointBindings

	// charms always return an empty binding in addition to the endpoints it provides
	if len(endpointBindings) <= 2 {
		var endpoint string
		//TODO: validate safety of deleting this item
		for val := range endpointBindings {
			if val == "" {
				delete(endpointBindings, val)
			} else {
				endpoint = val
			}
		}
		return fmt.Sprintf("%v:%v", applicationName, endpoint), nil
	}

	//TODO: reconcile with default juju behaviour that will match endpoints without specifying
	return "", fmt.Errorf("unable to discern a default endpoint for %s", applicationName)
}
