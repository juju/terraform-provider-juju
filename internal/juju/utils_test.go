// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLocalControllerConfig(t *testing.T) {
	config, err := parseLocalControllerConfig([]byte(`{
		"test-controller": {
			"details": {
				"api-endpoints": ["10.0.0.1:17070", "10.0.0.2:17070"],
				"agent-version": "3.6.0",
				"ca-cert": "test-ca-cert"
			},
			"account": {
				"user": "admin",
				"password": "super-secret"
			}
		}
	}`))
	require.NoError(t, err)

	assert.Equal(t, []string{"10.0.0.1:17070", "10.0.0.2:17070"}, config.ProviderDetails.ApiEndpoints)
	assert.Equal(t, "3.6.0", config.ProviderDetails.AgentVersion)
	assert.Equal(t, "test-ca-cert", config.ProviderDetails.CACert)
	assert.Equal(t, "admin", config.Account.User)
	assert.Equal(t, "super-secret", config.Account.Password)
}
