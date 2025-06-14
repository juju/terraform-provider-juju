// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"

	"github.com/juju/errors"
	cloudapi "github.com/juju/juju/api/client/cloud"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/names/v5"
)

type credentialsClient struct {
	SharedClient
}

type CreateCredentialInput struct {
	Attributes           map[string]string
	AuthType             string
	ClientCredential     bool
	CloudName            string
	ControllerCredential bool
	Name                 string
}

type CreateCredentialResponse struct {
	CloudCredential jujucloud.Credential
	CloudName       string
}

// ListCredentialResponse is the response from listing credentials.
type ListCredentialResponse struct {
	// CloudCredentials contain all credentials, keyed by cloud name.
	CloudCredentials map[string]jujucloud.CloudCredential
}

type ReadCredentialInput struct {
	Name                 string
	ClientCredential     bool
	CloudName            string
	ControllerCredential bool
}

type ReadCredentialResponse struct {
	CloudCredential jujucloud.Credential
}

type UpdateCredentialInput struct {
	Attributes           map[string]string
	AuthType             string
	ClientCredential     bool
	CloudName            string
	ControllerCredential bool
	Name                 string
}

type DestroyCredentialInput struct {
	ClientCredential     bool
	CloudName            string
	ControllerCredential bool
	Name                 string
}

func newCredentialsClient(sc SharedClient) *credentialsClient {
	return &credentialsClient{
		SharedClient: sc,
	}
}

func GetCloudCredentialTag(cloudName, currentUser, name string) (*names.CloudCredentialTag, error) {
	id := fmt.Sprintf("%s/%s/%s", cloudName, currentUser, name)
	if !names.IsValidCloudCredential(id) {
		return nil, fmt.Errorf("invalid cloud credential to cloud %s with user %s and credential name %s", cloudName, currentUser, name)
	}
	cloudCredentialTag := names.NewCloudCredentialTag(id)
	return &cloudCredentialTag, nil
}

// Based on:
// https://github.com/juju/juju/blob/develop/state/cloudcredentials.go#L388
func supportedAuth(cloud jujucloud.Cloud, authTypeReceived string) bool {
	for _, authType := range cloud.AuthTypes {
		if authTypeReceived == string(authType) {
			return true
		}
	}
	return false
}

func (c *credentialsClient) ValidateCredentialForCloud(cloudName, authTypeReceived string) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := cloudapi.NewClient(conn)

	cloudTag := names.NewCloudTag(cloudName)

	cloud, err := client.Cloud(cloudTag)
	if err != nil {
		return err
	}

	if !supportedAuth(cloud, authTypeReceived) {
		return errors.NotSupportedf("supported auth-types %q, %q", cloud.AuthTypes, authTypeReceived)
	}
	return nil
}

func (c *credentialsClient) CreateCredential(input CreateCredentialInput) (*CreateCredentialResponse, error) {
	if !input.ControllerCredential && !input.ClientCredential {
		// Just in case none of them are set
		return nil, fmt.Errorf("controller_credential or/and client_credential must be set to true")
	}

	credentialName := input.Name
	if !names.IsValidCloudCredentialName(credentialName) {
		return nil, errors.Errorf("%q is not a valid credential name", credentialName)
	}

	cloudName := input.CloudName

	if err := c.ValidateCredentialForCloud(cloudName, input.AuthType); err != nil {
		return nil, err
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := cloudapi.NewClient(conn)

	currentUser := getCurrentJujuUser(conn)

	cloudCredTag, err := GetCloudCredentialTag(cloudName, currentUser, credentialName)
	if err != nil {
		return nil, err
	}

	cloudCredential := jujucloud.NewNamedCredential(
		credentialName,
		jujucloud.AuthType(input.AuthType),
		input.Attributes,
		false,
	)

	if input.ClientCredential {
		if err := updateClientCredential(cloudName, credentialName, cloudCredential); err != nil {
			return nil, err
		}
	}

	if input.ControllerCredential {
		if err := client.AddCredential(cloudCredTag.String(), cloudCredential); err != nil {
			return nil, err
		}
	}

	return &CreateCredentialResponse{CloudCredential: cloudCredential, CloudName: cloudName}, nil
}

// ListClientCredentials lists all credentials on the client.
func (c *credentialsClient) ListClientCredentials() (*ListCredentialResponse, error) {
	clientCredentialFound, err := getExistingClientCredentials()
	if err != nil {
		return nil, err
	}
	return &ListCredentialResponse{
		CloudCredentials: clientCredentialFound,
	}, nil
}

// ListControllerCredentials lists all credentials on the controller.
func (c *credentialsClient) ListControllerCredentials() (*ListCredentialResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := cloudapi.NewClient(conn)

	controllerCredentialFound := map[string]jujucloud.CloudCredential{}
	credentialContents, err := client.CredentialContents("", "", true)
	if err != nil {
		return nil, err
	}

	for _, one := range credentialContents {
		remoteCredential := one.Result.Content
		cloudCredential, ok := controllerCredentialFound[remoteCredential.Cloud]
		if !ok {
			cloudCredential = jujucloud.CloudCredential{}
		}
		if cloudCredential.AuthCredentials == nil {
			cloudCredential.AuthCredentials = map[string]jujucloud.Credential{}
		}
		cloudCredential.AuthCredentials[remoteCredential.Name] = jujucloud.NewCredential(jujucloud.AuthType(remoteCredential.AuthType), remoteCredential.Attributes)
		controllerCredentialFound[remoteCredential.Cloud] = cloudCredential
	}

	return &ListCredentialResponse{
		CloudCredentials: controllerCredentialFound,
	}, nil
}

func (c *credentialsClient) ReadCredential(input ReadCredentialInput) (*ReadCredentialResponse, error) {
	clientCredential := input.ClientCredential
	cloudName := input.CloudName
	controllerCredential := input.ControllerCredential
	credentialName := input.Name

	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := cloudapi.NewClient(conn)

	var clientCredentialFound jujucloud.Credential
	if clientCredential {
		existingCredentials, err := getExistingClientCredential(cloudName)
		if err != nil {
			return nil, err
		}
		clientCredentialFound = existingCredentials.AuthCredentials[credentialName]
	}

	var controllerCredentialFound jujucloud.Credential
	if controllerCredential {
		credentialContents, err := client.CredentialContents(cloudName, credentialName, true)
		if err != nil {
			return nil, err
		}

		for _, content := range credentialContents {
			if content.Error != nil {
				continue
			}
			remoteCredential := content.Result.Content
			if remoteCredential.Name == credentialName {
				controllerCredentialFound = jujucloud.NewNamedCredential(
					credentialName,
					jujucloud.AuthType(remoteCredential.AuthType),
					remoteCredential.Attributes,
					false, //  CredentialContents does not provides this field
				)
				break
			}
		}
	}

	if controllerCredential && clientCredential {
		// compare if they are the same
		// lets just check auth_type for now
		if clientCredentialFound.AuthType() != controllerCredentialFound.AuthType() {
			return nil, fmt.Errorf("client and controller credentials have different auth type: %s, %s", clientCredentialFound.AuthType(), controllerCredentialFound.AuthType())
		}
	}

	if controllerCredential {
		return &ReadCredentialResponse{
			CloudCredential: controllerCredentialFound,
		}, nil
	}

	if clientCredential {
		return &ReadCredentialResponse{
			CloudCredential: clientCredentialFound,
		}, nil
	}

	return nil, fmt.Errorf("credential %s not found for cloud %s", credentialName, cloudName)
}

func (c *credentialsClient) UpdateCredential(input UpdateCredentialInput) error {
	if !input.ControllerCredential && !input.ClientCredential {
		// Just in case none of them are set
		return fmt.Errorf("controller_credential or/and client_credential must be set to true")
	}

	credentialName := input.Name
	if !names.IsValidCloudCredentialName(credentialName) {
		return errors.Errorf("%q is not a valid credential name", credentialName)
	}

	cloudName := input.CloudName

	if err := c.ValidateCredentialForCloud(cloudName, input.AuthType); err != nil {
		return err
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	currentUser := getCurrentJujuUser(conn)

	cloudCredTag, err := GetCloudCredentialTag(cloudName, currentUser, credentialName)
	if err != nil {
		return err
	}

	cloudCredential := jujucloud.NewNamedCredential(
		input.Name,
		jujucloud.AuthType(input.AuthType),
		input.Attributes,
		false,
	)

	if input.ClientCredential {
		if err := updateClientCredential(cloudName, credentialName, cloudCredential); err != nil {
			return err
		}
	}

	if input.ControllerCredential {
		client := cloudapi.NewClient(conn)

		if _, err := client.UpdateCredentialsCheckModels(*cloudCredTag, cloudCredential); err != nil {
			return err
		}
	}

	return nil
}

func getExistingClientCredentials() (map[string]jujucloud.CloudCredential, error) {
	store := jujuclient.NewFileClientStore()
	existingCredentials, err := store.AllCredentials()
	if err != nil {
		return nil, errors.Annotate(err, "reading all existing credentials")
	}
	return existingCredentials, nil
}

func getExistingClientCredential(cloudName string) (*jujucloud.CloudCredential, error) {
	store := jujuclient.NewFileClientStore()
	existingCredentials, err := store.CredentialForCloud(cloudName)
	if err != nil && !errors.Is(err, errors.NotFound) {
		return nil, errors.Annotate(err, "reading existing credentials for cloud")
	}
	if errors.Is(err, errors.NotFound) {
		return nil, fmt.Errorf("credential not found for cloud %s: %s", cloudName, err)
	}
	return existingCredentials, nil
}

func updateClientCredential(cloudName string, credentialName string, cloudCredential jujucloud.Credential) error {
	existingCredentials, err := getExistingClientCredential(cloudName)
	if err != nil {
		return err
	}
	// will overwrite if already exists
	existingCredentials.AuthCredentials[credentialName] = cloudCredential
	store := jujuclient.NewFileClientStore()
	if err := store.UpdateCredential(cloudName, *existingCredentials); err != nil {
		return fmt.Errorf("credential %s not added for cloud %s: %s", credentialName, cloudName, err)
	}
	return nil
}

func (c *credentialsClient) DestroyCredential(input DestroyCredentialInput) error {
	cloudName := input.CloudName
	credentialName := input.Name

	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := cloudapi.NewClient(conn)

	currentUser := getCurrentJujuUser(conn)

	cloudCredTag, err := GetCloudCredentialTag(cloudName, currentUser, credentialName)
	if err != nil {
		return err
	}

	if input.ControllerCredential {
		if err := client.RevokeCredential(*cloudCredTag, false); err != nil {
			return err
		}
	}

	if input.ClientCredential {
		if err := destroyClientCredential(cloudName, credentialName); err != nil {
			return err
		}
	}

	return nil
}

func destroyClientCredential(cloudName string, credentialName string) error {
	existingCredentials, err := getExistingClientCredential(cloudName)
	if err != nil {
		return err
	}
	if _, ok := existingCredentials.AuthCredentials[credentialName]; !ok {
		return fmt.Errorf("credential %s not found for cloud %s", credentialName, cloudName)
	}
	delete(existingCredentials.AuthCredentials, credentialName)
	store := jujuclient.NewFileClientStore()
	if err := store.UpdateCredential(cloudName, *existingCredentials); err != nil {
		return fmt.Errorf("credential %s not deleted for cloud %s: %s", credentialName, cloudName, err)
	}
	return nil
}
