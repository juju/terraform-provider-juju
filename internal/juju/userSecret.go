// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"strings"

	"github.com/juju/juju/api/client/secrets"
)

type userSecretClient struct {
	SharedClient
}

type AddUserSecretInput struct {
	ModelName   string
	Name        string
	Value       string
	Description string
}

type AddUserSecretOutput struct {
	SecretURI string
}

type ReadUserSecretInput struct {
	ModelName string
	Name      string
}

type UpdateUserSecretInput struct {
	ModelName string
	Name      string
	Value     string
}

type RemoveUserSecretInput struct {
	ModelName string
	Name      string
}

func newUserSecretClient(sc SharedClient) *userSecretClient {
	return &userSecretClient{
		SharedClient: sc,
	}
}

func (c *userSecretClient) AddUserSecret(input *AddUserSecretInput) (*AddUserSecretOutput, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := secrets.NewClient(conn)
	secretURI, err := client.CreateSecret(input.Name, input.Description, parseSecretValueStringToMap(input.Value))
	if err != nil {
		return nil, err
	}
	return &AddUserSecretOutput{
		SecretURI: secretURI,
	}, nil
}

func parseSecretValueStringToMap(input string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(input, " ")

	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}

	return result
}
