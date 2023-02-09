package juju

import (
	"fmt"
	"strings"

	"github.com/juju/juju/core/constraints"

	cloudapi "github.com/juju/juju/api/client/cloud"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type credentialsClient struct {
	ConnectionFactory
}

type CreateCredentialInput struct {
	Name       string
	CloudList  []interface{}
	AuthType   string
	Attributes map[string]string
}

type CreateCredentialResponse struct {
	CloudCredential jujucloud.Credential
}

type ReadCredentialInput struct {
	UUID string
}

type ReadCredentialResponse struct {
	ModelInfo        params.ModelInfo
	ModelConfig      map[string]interface{}
	ModelConstraints constraints.Value
}

type UpdateCredentialInput struct {
	UUID        string
	Config      map[string]interface{}
	Unset       []string
	Constraints *constraints.Value
}

type DestroyCredentialInput struct {
	UUID string
}

func newCredentialsClient(cf ConnectionFactory) *credentialsClient {
	return &credentialsClient{
		ConnectionFactory: cf,
	}
}

func (c *credentialsClient) CreateCredential(input CreateCredentialInput) (*CreateCredentialResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	client := cloudapi.NewClient(conn)
	defer client.Close()

	var cloudName string
	for _, cloud := range input.CloudList {
		cloudMap := cloud.(map[string]interface{})
		cloudName = cloudMap["name"].(string)
	}

	currentUser := strings.TrimPrefix(conn.AuthTag().String(), PrefixUser)

	id := fmt.Sprintf("%s/%s/%s", cloudName, currentUser, input.Name)
	if !names.IsValidCloudCredential(id) {
		return nil, err
	}
	cloudCredTag := names.NewCloudCredentialTag(id)

	cloudCredential := jujucloud.NewNamedCredential(
		input.Name,
		jujucloud.AuthType(input.AuthType),
		input.Attributes,
		false,
	)

	if err := client.AddCredential(cloudCredTag.String(), cloudCredential); err != nil {
		return nil, err
	}

	return &CreateCredentialResponse{CloudCredential: cloudCredential}, nil
}
