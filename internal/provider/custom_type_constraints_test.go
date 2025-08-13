// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestCustomConstraintsValue_StringSemanticEquals(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name      string
		left      string
		right     string
		wantEqual bool
		wantError bool
	}{
		{
			name:      "identical strings",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "different order, semantically equal",
			left:      "mem=4G cpu-cores=2",
			right:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "different values",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=4 mem=4G",
			wantEqual: false,
		},
		{
			name:      "different memory values, semantically equal",
			left:      "cpu-cores=2 mem=4096M",
			right:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "extra constraint",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=2 mem=4G root-disk=10G",
			wantEqual: false,
		},
		{
			name:      "auto-added constraint key missing on one side",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=2 mem=4G arch=amd64",
			wantEqual: true,
		},
		{
			name:      "auto-added constraint key present but different",
			left:      "cpu-cores=2 mem=4G arch=amd64",
			right:     "cpu-cores=2 mem=4G arch=arm64",
			wantEqual: false,
		},
		{
			name:      "malformed left constraint",
			left:      "cpu-cores=2 mem=4G badtoken",
			right:     "cpu-cores=2 mem=4G",
			wantError: true,
		},
		{
			name:      "malformed right constraint",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=2 mem=4G badtoken",
			wantError: true,
		},
		{
			name:      "completely invalid left constraint",
			left:      "!!!",
			right:     "cpu-cores=2",
			wantError: true,
		},
		{
			name:      "completely invalid right constraint",
			left:      "cpu-cores=2",
			right:     "!!!",
			wantError: true,
		},
	}

	for _, tt := range tests {
		left := NewCustomConstraintsValue(tt.left)
		right := NewCustomConstraintsValue(tt.right)
		for i := range 2 {
			// Reverse the order for the second iteration
			// to test both directions of comparison.
			var name string
			if i == 0 {
				name = tt.name + "/forward"
			} else {
				name = tt.name + "/reverse"
				left, right = right, left // swap for reverse test
			}
			t.Run(name, func(t *testing.T) {
				equal, diags := left.StringSemanticEquals(ctx, right)
				assert.Equal(t, tt.wantEqual, equal, "equality mismatch for %s", name)
				if tt.wantError {
					assert.True(t, diags.HasError())
				} else {
					assert.False(t, diags.HasError())
				}
			})
		}
	}
}

func TestCustomConstraintsType_TypeMistmatch(t *testing.T) {
	left := NewCustomConstraintsValue("cpu-cores=2")
	other := basetypes.NewStringValue("cpu-cores=2")
	equal, diags := left.StringSemanticEquals(t.Context(), other)
	assert.False(t, equal)
	assert.True(t, diags.HasError())
}
