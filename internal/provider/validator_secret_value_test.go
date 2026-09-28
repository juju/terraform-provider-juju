// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

// TestValidateSecretValueMap covers the shared null-element check used by the
// plan-time SecretValueMapValidator and the apply-time secretValueMap. It
// confirms a null element is rejected, while unknown elements (e.g. a value_wo
// element fed by an ephemeral variable resolved only at apply), known strings, a
// null map and an unknown map are all tolerated.
func TestValidateSecretValueMap(t *testing.T) {
	basePath := path.Root("value_wo")

	tests := []struct {
		name        string
		value       types.Map
		expectError bool
	}{
		{
			name:        "null element rejected",
			value:       types.MapValueMust(types.StringType, map[string]attr.Value{"password": types.StringNull()}),
			expectError: true,
		},
		{
			name:        "unknown element tolerated",
			value:       types.MapValueMust(types.StringType, map[string]attr.Value{"password": types.StringUnknown()}),
			expectError: false,
		},
		{
			name:        "known element accepted",
			value:       types.MapValueMust(types.StringType, map[string]attr.Value{"password": types.StringValue("s3cret")}),
			expectError: false,
		},
		{
			name:        "mixed known and null rejected",
			value:       types.MapValueMust(types.StringType, map[string]attr.Value{"ok": types.StringValue("v"), "bad": types.StringNull()}),
			expectError: true,
		},
		{
			name:        "null map tolerated",
			value:       types.MapNull(types.StringType),
			expectError: false,
		},
		{
			name:        "unknown map tolerated",
			value:       types.MapUnknown(types.StringType),
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := validateSecretValueMap(tt.value, basePath)
			if tt.expectError {
				assert.True(t, diags.HasError(), "expected an error diagnostic, got: %v", diags)
				found := false
				for _, d := range diags.Errors() {
					if d.Summary() == "Null Secret Value" {
						found = true
					}
				}
				assert.True(t, found, "expected a 'Null Secret Value' diagnostic, got: %v", diags)
			} else {
				assert.False(t, diags.HasError(), "expected no error diagnostics, got: %v", diags)
			}
		})
	}
}
