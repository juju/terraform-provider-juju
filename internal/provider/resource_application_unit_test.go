// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"errors"
	"testing"

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
		name     string
		plan     juju.CharmResources
		state    juju.CharmResources
		expected juju.CharmResources
	}{
		{
			name:     "unchanged OCI credentials are not re-uploaded",
			plan:     juju.CharmResources{"image": image},
			state:    juju.CharmResources{"image": image},
			expected: juju.CharmResources{},
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
			require.Equal(t, tt.expected, computeResourceUpdates(tt.plan, tt.state))
		})
	}
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
