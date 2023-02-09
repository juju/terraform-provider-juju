package juju

import (
	"fmt"
	"strings"

	cloudapi "github.com/juju/juju/api/client/cloud"
	jujucloud "github.com/juju/juju/cloud"
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
	CloudName       string
}

type ReadCredentialInput struct {
	Name string
}

type ReadCredentialResponse struct {
	CloudCredential jujucloud.Credential
}

type UpdateCredentialInput struct {
	Name       string
	AuthType   string
	Attributes map[string]string
}

type DestroyCredentialInput struct {
	Name string
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

	return &CreateCredentialResponse{CloudCredential: cloudCredential, CloudName: cloudName}, nil
}

func (c *credentialsClient) ReadCredential(credentialName, cloudName string) (*ReadCredentialResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	client := cloudapi.NewClient(conn)
	defer client.Close()

	credentialContents, err := client.CredentialContents(cloudName, credentialName, false)
	if err != nil {
		return nil, err
	}

	for _, content := range credentialContents {
		if content.Error != nil {
			continue
		}
		remoteCredential := content.Result.Content
		if remoteCredential.Name == credentialName {
			cloudCredential := jujucloud.NewNamedCredential(
				credentialName,
				jujucloud.AuthType(remoteCredential.AuthType),
				remoteCredential.Attributes,
				*remoteCredential.Valid, // to be confirmed if corresponds to revoked
			)
			return &ReadCredentialResponse{
				CloudCredential: cloudCredential,
			}, nil
		}
	}

	return nil, fmt.Errorf("credential %s not found for cloud %s", credentialName, cloudName)
}

func (c *credentialsClient) UpdateCredential(input UpdateCredentialInput) error {
	return nil
}
