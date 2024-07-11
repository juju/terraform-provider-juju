// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"encoding/base64"
	"errors"
	"strings"

	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api"
	apisecrets "github.com/juju/juju/api/client/secrets"
	coresecrets "github.com/juju/juju/core/secrets"
)

type secretsClient struct {
	SharedClient

	getSecretAPIClient func(connection api.Connection) SecretAPIClient
}

type AccessSecretAction int

const (
	GrantAccess AccessSecretAction = iota
	RevokeAccess
)

type CreateSecretInput struct {
	ModelName string
	Name      string
	Value     map[string]string
	Info      string
}

type CreateSecretOutput struct {
	SecretId string
}

type ReadSecretInput struct {
	SecretId  string
	ModelName string
	Name      *string
	Revision  *int
}

type ReadSecretOutput struct {
	SecretId     string
	Name         string
	Value        map[string]string
	Applications []string
	Info         string
}

type UpdateSecretInput struct {
	SecretId  string
	ModelName string
	Name      *string
	Value     *map[string]string
	AutoPrune *bool
	Info      *string
}

type DeleteSecretInput struct {
	SecretId  string
	ModelName string
}

type GrantRevokeAccessSecretInput struct {
	SecretId     string
	ModelName    string
	Applications []string
}

type MultiError struct {
	Errors []error
}

// Error returns a string representation of the MultiError.
func (m *MultiError) Error() string {
	errStrs := make([]string, 0, len(m.Errors))
	for _, err := range m.Errors {
		errStrs = append(errStrs, err.Error())
	}
	return strings.Join(errStrs, ", ")
}

func newSecretsClient(sc SharedClient) *secretsClient {
	return &secretsClient{
		SharedClient: sc,
		getSecretAPIClient: func(connection api.Connection) SecretAPIClient {
			return apisecrets.NewClient(connection)
		},
	}
}

// CreateSecret creates a new secret.
func (c *secretsClient) CreateSecret(input *CreateSecretInput) (CreateSecretOutput, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return CreateSecretOutput{}, err
	}
	defer func() { _ = conn.Close() }()

	secretAPIClient := c.getSecretAPIClient(conn)

	// Encode the secret values as base64
	encodedValue := make(map[string]string, len(input.Value))
	for k, v := range input.Value {
		encodedValue[k] = base64.StdEncoding.EncodeToString([]byte(v))
	}

	secretId, err := secretAPIClient.CreateSecret(input.Name, input.Info, encodedValue)
	if err != nil {
		return CreateSecretOutput{}, typedError(err)
	}
	secretURI, err := coresecrets.ParseURI(secretId)
	if err != nil {
		return CreateSecretOutput{}, typedError(err)
	}
	return CreateSecretOutput{
		SecretId: secretURI.ID,
	}, nil
}

// ReadSecret reads a secret.
func (c *secretsClient) ReadSecret(input *ReadSecretInput) (ReadSecretOutput, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return ReadSecretOutput{}, err
	}
	defer func() { _ = conn.Close() }()

	secretAPIClient := c.getSecretAPIClient(conn)

	var secretURI *coresecrets.URI
	if input.SecretId != "" {
		secretURI, err = coresecrets.ParseURI(input.SecretId)
		if err != nil {
			return ReadSecretOutput{}, err
		}
	} else {
		secretURI = nil
	}
	secretFilter := coresecrets.Filter{
		URI:      secretURI,
		Label:    input.Name,
		Revision: input.Revision,
	}

	results, err := secretAPIClient.ListSecrets(true, secretFilter)
	if err != nil {
		return ReadSecretOutput{}, typedError(err)
	}
	if len(results) < 1 {
		return ReadSecretOutput{}, &secretNotFoundError{secretId: input.SecretId}
	}
	if results[0].Error != "" {
		return ReadSecretOutput{}, errors.New(results[0].Error)
	}

	// Decode the secret values from base64
	decodedValue, err := results[0].Value.Values()
	if err != nil {
		return ReadSecretOutput{}, err
	}

	// Get applications from Access info
	applications := getApplicationsFromAccessInfo(results[0].Access)

	return ReadSecretOutput{
		SecretId:     results[0].Metadata.URI.ID,
		Name:         results[0].Metadata.Label,
		Value:        decodedValue,
		Applications: applications,
		Info:         results[0].Metadata.Description,
	}, nil
}

// UpdateSecret updates a secret.
func (c *secretsClient) UpdateSecret(input *UpdateSecretInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	secretAPIClient := c.getSecretAPIClient(conn)

	// Specify by ID or Name
	if input.SecretId == "" && input.Name == nil {
		return errors.New("must specify either secret ID or name")
	}

	// Define default values
	var info string
	if input.Info != nil {
		info = *input.Info
	} else {
		info = ""
	}
	var value map[string]string
	if input.Value != nil {
		// Encode the secret values as base64
		encodedValue := make(map[string]string, len(*input.Value))
		for k, v := range *input.Value {
			encodedValue[k] = base64.StdEncoding.EncodeToString([]byte(v))
		}

		value = encodedValue
	} else {
		value = map[string]string{}
	}

	if input.SecretId != "" {
		// Specify by ID
		secretURI, err := coresecrets.ParseURI(input.SecretId)
		if err != nil {
			return err
		}
		if input.Name == nil {
			// Update secret without changing the name
			err = secretAPIClient.UpdateSecret(secretURI, "", input.AutoPrune, "", info, value)
			if err != nil {
				return typedError(err)
			}
		} else {
			// Update secret with a new name
			err = secretAPIClient.UpdateSecret(secretURI, "", input.AutoPrune, *input.Name, info, value)
			if err != nil {
				return typedError(err)
			}
		}
	} else {
		return errors.New("updating secrets by name is not supported")
	}

	return nil
}

// DeleteSecret deletes a secret.
func (c *secretsClient) DeleteSecret(input *DeleteSecretInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}

	secretAPIClient := c.getSecretAPIClient(conn)
	secretURI, err := coresecrets.ParseURI(input.SecretId)
	if err != nil {
		return err
	}
	// TODO: think about removing concrete revision.
	err = secretAPIClient.RemoveSecret(secretURI, "", nil)
	if !errors.Is(err, jujuerrors.NotFound) {
		return typedError(err)
	}

	return nil
}

// UpdateAccessSecret updates access to a secret.
func (c *secretsClient) UpdateAccessSecret(input *GrantRevokeAccessSecretInput, op AccessSecretAction) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	secretAPIClient := c.getSecretAPIClient(conn)

	secretURI, err := coresecrets.ParseURI(input.SecretId)
	if err != nil {
		return err
	}

	var results []error
	switch op {
	case GrantAccess:
		results, err = secretAPIClient.GrantSecret(secretURI, "", input.Applications)
	case RevokeAccess:
		results, err = secretAPIClient.RevokeSecret(secretURI, "", input.Applications)
	default:
		return errors.New("invalid op")
	}

	if err != nil {
		return typedError(err)
	}
	return errors.Join(results...)
}

// getApplicationsFromAccessInfo returns a list of applications from the access info.
func getApplicationsFromAccessInfo(accessInfo []coresecrets.AccessInfo) []string {
	applications := make([]string, 0, len(accessInfo))
	for _, info := range accessInfo {
		// Trim the prefix "application-" from the application name (info.Target)
		applications = append(applications, strings.TrimPrefix(info.Target, PrefixApplication))
	}
	return applications
}
