// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestComputeResourceUpdates(t *testing.T) {
	image := juju.CharmResource{
		OCIImageURL:      "registry.example.com/path/image:tag",
		RegistryUser:     "user",
		RegistryPassword: "password",
	}
	anonymousImage := juju.CharmResource{OCIImageURL: image.OCIImageURL}
	changedImage := image
	changedImage.OCIImageURL = "registry.example.com/path/image:new"
	rotatedImage := image
	rotatedImage.RegistryPassword = "new-password"
	pinned := juju.CharmResource{RevisionNumber: "42"}

	tests := []struct {
		name                      string
		plan                      juju.CharmResources
		state                     juju.CharmResources
		credentialsVersionChanged bool
		expected                  juju.CharmResources
	}{
		{
			name:     "unchanged OCI credentials are not re-uploaded",
			plan:     juju.CharmResources{"image": image},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{},
		},
		{
			name:     "empty write-only credentials with unchanged version are not re-uploaded",
			plan:     juju.CharmResources{"image": anonymousImage},
			state:    juju.CharmResources{"image": anonymousImage},
			expected: juju.CharmResources{},
		},
		{
			name:                      "version bump re-sends credentials even when comparison is equal",
			plan:                      juju.CharmResources{"image": image},
			state:                     juju.CharmResources{"image": image},
			credentialsVersionChanged: true,
			expected:                  juju.CharmResources{"image": image},
		},
		{
			name:                      "version bump to empty credentials clears authentication",
			plan:                      juju.CharmResources{"image": anonymousImage},
			state:                     juju.CharmResources{"image": anonymousImage},
			credentialsVersionChanged: true,
			expected:                  juju.CharmResources{"image": anonymousImage},
		},
		{
			name:                      "removing write-only version clears authentication",
			plan:                      juju.CharmResources{"image": anonymousImage},
			state:                     juju.CharmResources{"image": anonymousImage},
			credentialsVersionChanged: true,
			expected:                  juju.CharmResources{"image": anonymousImage},
		},
		{
			name:     "plain credential rotation re-uploads image",
			plan:     juju.CharmResources{"image": rotatedImage},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{"image": rotatedImage},
		},
		{
			name:     "removing plain credentials clears authentication",
			plan:     juju.CharmResources{"image": anonymousImage},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{"image": anonymousImage},
		},
		{
			name:     "changed image URL uses current credentials",
			plan:     juju.CharmResources{"image": changedImage},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{"image": changedImage},
		},
		{
			name:     "added image uses current credentials",
			plan:     juju.CharmResources{"image": image},
			expected: juju.CharmResources{"image": image},
		},
		{
			name:     "unchanged numeric revision stays pinned",
			plan:     juju.CharmResources{"file": pinned, "image": image},
			state:    juju.CharmResources{"file": pinned, "image": image},
			expected: juju.CharmResources{"file": pinned},
		},
		{
			name:     "numeric revision stays pinned when another resource changes",
			plan:     juju.CharmResources{"file": pinned, "image": changedImage},
			state:    juju.CharmResources{"file": pinned, "image": image},
			expected: juju.CharmResources{"file": pinned, "image": changedImage},
		},
		{
			name:     "removed resource resets to latest without uploading unchanged image",
			plan:     juju.CharmResources{"image": image},
			state:    juju.CharmResources{"file": pinned, "image": image},
			expected: juju.CharmResources{"file": {RevisionNumber: "-1"}},
		},
		{
			name:     "image can be replaced by numeric revision",
			plan:     juju.CharmResources{"image": pinned},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{"image": pinned},
		},
		{
			name:     "numeric revision can be replaced by image",
			plan:     juju.CharmResources{"image": image},
			state:    juju.CharmResources{"image": pinned},
			expected: juju.CharmResources{"image": image},
		},
		{
			name:     "no resources",
			expected: juju.CharmResources{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.expected, computeResourceUpdates(tt.plan, tt.state, tt.credentialsVersionChanged))
		})
	}
}

func TestResolveRegistryCredentials(t *testing.T) {
	ctx := t.Context()
	credentials := map[string]registryDetails{
		"registry.example.com/path": {
			User:     types.StringValue("user"),
			Password: types.StringValue("password"),
		},
	}
	credentialType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"username": types.StringType,
		"password": types.StringType,
	}}
	configCredentials, diags := types.MapValueFrom(ctx, credentialType, credentials)
	require.False(t, diags.HasError(), "%v", diags)
	configValue := types.ObjectValueMust(
		map[string]attr.Type{"registry_credentials_wo": configCredentials.Type(ctx)},
		map[string]attr.Value{"registry_credentials_wo": configCredentials},
	)
	raw, err := configValue.ToTerraformValue(ctx)
	require.NoError(t, err)
	var schemaResp resource.SchemaResponse
	(&applicationResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	config := tfsdk.Config{
		Raw: raw,
		Schema: schema.Schema{Attributes: map[string]schema.Attribute{
			"registry_credentials_wo": schemaResp.Schema.Attributes["registry_credentials_wo"],
		}},
	}

	t.Run("write-only credentials come from configuration", func(t *testing.T) {
		actual, diags := resolveRegistryCredentials(ctx, applicationResourceModelV1{
			RegistryCredentialsWOVersion: types.Int64Value(1),
		}, config)
		require.False(t, diags.HasError(), "%v", diags)
		require.Equal(t, credentials, actual)
	})
	t.Run("plain credentials come from plan", func(t *testing.T) {
		actual, diags := resolveRegistryCredentials(ctx, applicationResourceModelV1{
			RegistryCredentials:          credentials,
			RegistryCredentialsWOVersion: types.Int64Null(),
		}, tfsdk.Config{})
		require.False(t, diags.HasError(), "%v", diags)
		require.Equal(t, credentials, actual)
	})
}

func TestAssertEqualsUnitCount(t *testing.T) {
	testCases := []struct {
		name        string
		units       int
		response    *juju.ReadApplicationResponse
		expectError bool
	}{
		{
			name:  "units match",
			units: 1,
			response: &juju.ReadApplicationResponse{
				Principal: true,
				Units:     1,
			},
			expectError: false,
		},
		{
			name:  "units mismatch",
			units: 1,
			response: &juju.ReadApplicationResponse{
				Principal: true,
				Units:     0,
			},
			expectError: true,
		},
		{
			name:  "zero units and no machines",
			units: 0,
			response: &juju.ReadApplicationResponse{
				Principal: true,
				Units:     0,
				Machines:  []string{},
			},
			expectError: false,
		},
		{
			name:  "zero units but machines still present",
			units: 0,
			response: &juju.ReadApplicationResponse{
				Principal: true,
				Units:     0,
				Machines:  []string{"0"},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertEqualsUnitCount(tc.units)(tc.response)
			if tc.expectError {
				require.Error(t, err)
				require.True(t, errors.Is(err, juju.RetryReadError), "expected RetryReadError, got %v", err)
				return
			}

			require.NoError(t, err)
		})
	}
}
