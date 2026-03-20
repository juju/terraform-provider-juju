// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	jujuerrors "github.com/juju/errors"
	"github.com/stretchr/testify/require"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

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
				Units: 1,
			},
			expectError: false,
		},
		{
			name:  "units mismatch",
			units: 1,
			response: &juju.ReadApplicationResponse{
				Units: 0,
			},
			expectError: true,
		},
		{
			name:  "zero units and no machines",
			units: 0,
			response: &juju.ReadApplicationResponse{
				Units:    0,
				Machines: []string{},
			},
			expectError: false,
		},
		{
			name:  "zero units but machines still present",
			units: 0,
			response: &juju.ReadApplicationResponse{
				Units:    0,
				Machines: []string{"0"},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := assertEqualsUnitCount(tc.units)(tc.response)
			if tc.expectError {
				require.Error(t, err)
				require.True(t, jujuerrors.Is(err, juju.RetryReadError), "expected RetryReadError, got %v", err)
				return
			}

			require.NoError(t, err)
		})
	}
}
