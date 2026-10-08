// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestNewNormalizedCustomConstraintsValue(t *testing.T) {
	t.Run("empty string becomes null", func(t *testing.T) {
		value := NewNormalizedCustomConstraintsValue("")
		assert.True(t, value.IsNull())
	})

	t.Run("non-empty string stays non-null", func(t *testing.T) {
		value := NewNormalizedCustomConstraintsValue("mem=512M")
		assert.False(t, value.IsNull())
		assert.Equal(t, "mem=512M", value.ValueString())
	})
}

func TestCustomConstraintsValue_StringSemanticEquals(t *testing.T) {
	ctx := t.Context()

	// actual is the value reported by Juju (the receiver) and prior is the
	// planned or prior state value (the argument), matching how the
	// framework calls StringSemanticEquals.
	tests := []struct {
		name      string
		actual    string
		prior     string
		wantEqual bool
		wantError bool
	}{
		{
			name:      "identical strings",
			actual:    "cpu-cores=2 mem=4G",
			prior:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "different order, semantically equal",
			actual:    "mem=4G cpu-cores=2",
			prior:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "different values",
			actual:    "cpu-cores=2 mem=4G",
			prior:     "cpu-cores=4 mem=4G",
			wantEqual: false,
		},
		{
			name:      "different memory values, semantically equal",
			actual:    "cpu-cores=2 mem=4096M",
			prior:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "extra constraint reported by juju",
			actual:    "cpu-cores=2 mem=4G root-disk=10G",
			prior:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "prior constraint missing from juju",
			actual:    "cpu-cores=2 mem=4G",
			prior:     "cpu-cores=2 mem=4G root-disk=10G",
			wantEqual: false,
		},
		{
			name:      "model constraints merged into machine constraints",
			actual:    "arch=amd64 cores=1 mem=2048M root-disk=10240M root-disk-source=volume",
			prior:     "arch=amd64 cores=1 mem=2048M root-disk=10240M",
			wantEqual: true,
		},
		{
			name:      "model constraints merged into application constraints",
			actual:    "arch=amd64 cores=1 mem=4096M root-disk-source=default",
			prior:     "mem=4G",
			wantEqual: true,
		},
		{
			name:      "model constraint overridden by prior",
			actual:    "arch=amd64 cores=2 mem=4096M root-disk-source=default",
			prior:     "mem=4G cores=2",
			wantEqual: true,
		},
		{
			name:      "arch added by juju",
			actual:    "cpu-cores=2 mem=4G arch=amd64",
			prior:     "cpu-cores=2 mem=4G",
			wantEqual: true,
		},
		{
			name:      "arch dropped by juju",
			actual:    "cpu-cores=2 mem=4G",
			prior:     "cpu-cores=2 mem=4G arch=amd64",
			wantEqual: true,
		},
		{
			name:      "arch present but different",
			actual:    "cpu-cores=2 mem=4G arch=arm64",
			prior:     "cpu-cores=2 mem=4G arch=amd64",
			wantEqual: false,
		},
		{
			name:      "empty prior",
			actual:    "arch=amd64 mem=4G",
			prior:     "",
			wantEqual: true,
		},
		{
			name:      "empty actual",
			actual:    "",
			prior:     "mem=4G",
			wantEqual: false,
		},
		{
			name:      "malformed actual constraint",
			actual:    "cpu-cores=2 mem=4G badtoken",
			prior:     "cpu-cores=2 mem=4G",
			wantError: true,
		},
		{
			name:      "malformed prior constraint",
			actual:    "cpu-cores=2 mem=4G",
			prior:     "cpu-cores=2 mem=4G badtoken",
			wantError: true,
		},
		{
			name:      "completely invalid actual constraint",
			actual:    "!!!",
			prior:     "cpu-cores=2",
			wantError: true,
		},
		{
			name:      "completely invalid prior constraint",
			actual:    "cpu-cores=2",
			prior:     "!!!",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := NewCustomConstraintsValue(tt.actual)
			prior := NewCustomConstraintsValue(tt.prior)
			equal, diags := actual.StringSemanticEquals(ctx, prior)
			assert.Equal(t, tt.wantEqual, equal)
			if tt.wantError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}
		})
	}
}

func TestConstraintsEqual(t *testing.T) {
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
			name:      "arch missing on one side",
			left:      "cpu-cores=2 mem=4G",
			right:     "cpu-cores=2 mem=4G arch=amd64",
			wantEqual: true,
		},
		{
			name:      "arch present but different",
			left:      "cpu-cores=2 mem=4G arch=amd64",
			right:     "cpu-cores=2 mem=4G arch=arm64",
			wantEqual: false,
		},
		{
			name:      "malformed constraint",
			left:      "cpu-cores=2 mem=4G badtoken",
			right:     "cpu-cores=2 mem=4G",
			wantError: true,
		},
		{
			name:      "completely invalid constraint",
			left:      "!!!",
			right:     "cpu-cores=2",
			wantError: true,
		},
	}

	for _, tt := range tests {
		// Run each case in both directions, as the comparison is symmetric.
		for _, dir := range []struct {
			name string
			a, b string
		}{
			{"forward", tt.left, tt.right},
			{"reverse", tt.right, tt.left},
		} {
			t.Run(tt.name+"/"+dir.name, func(t *testing.T) {
				equal, diags := constraintsEqual(dir.a, dir.b)
				assert.Equal(t, tt.wantEqual, equal)
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
