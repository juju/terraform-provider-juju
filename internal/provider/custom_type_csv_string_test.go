// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestCustomCommaDelimitedStringValue_StringSemanticEquals(t *testing.T) {
	ctx := t.Context()

	tests := []struct {
		name      string
		left      string
		right     string
		wantEqual bool
		wantError bool
	}{{
		name:      "identical strings",
		left:      "a,b,c",
		right:     "a,b,c",
		wantEqual: true,
	}, {
		name:      "different order, semantically equal",
		left:      "a,b,c",
		right:     "c,b,a",
		wantEqual: true,
	}, {
		name:      "different values",
		left:      "a,b",
		right:     "a,b,c",
		wantEqual: false,
	}, {
		name:      "different values",
		left:      "a,b,d",
		right:     "a,b,c",
		wantEqual: false,
	}}

	for _, tt := range tests {
		left := NewCustomCommaDelimitedStringValue(tt.left)
		right := NewCustomCommaDelimitedStringValue(tt.right)

		t.Run(tt.name, func(t *testing.T) {
			equal, diags := left.StringSemanticEquals(ctx, right)
			assert.Equal(t, tt.wantEqual, equal, "equality mismatch for %s", tt.name)
			if tt.wantError {
				assert.True(t, diags.HasError())
			} else {
				assert.False(t, diags.HasError())
			}
		})
	}
}

func TestCustomCommaDelimitedStringType_TypeMistmatch(t *testing.T) {
	left := NewCustomCommaDelimitedStringValue("a,b,c")
	other := basetypes.NewStringValue("a,b,c")
	equal, diags := left.StringSemanticEquals(t.Context(), other)
	assert.False(t, equal)
	assert.True(t, diags.HasError())
}
