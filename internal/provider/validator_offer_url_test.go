// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

func TestValidatorOfferURL(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		value       types.String
		expectError bool
	}

	tests := []testCase{
		{
			name:        "valid offer URL without controller",
			value:       types.StringValue("admin/model.offer"),
			expectError: false,
		},
		{
			name:        "valid offer URL with non-admin user",
			value:       types.StringValue("user/modelname.offername"),
			expectError: false,
		},
		{
			name:        "invalid offer URL with controller source",
			value:       types.StringValue("controller:admin/model.offer"),
			expectError: true,
		},
		{
			name:        "invalid offer URL with complex controller source",
			value:       types.StringValue("microcloud-core:admin/cos.prometheus-receive-remote-write"),
			expectError: true,
		},
		{
			name:        "invalid malformed offer URL",
			value:       types.StringValue("invalid-url"),
			expectError: true,
		},
		{
			name:        "null value",
			value:       types.StringNull(),
			expectError: false,
		},
		{
			name:        "unknown value",
			value:       types.StringUnknown(),
			expectError: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			request := validator.StringRequest{
				Path:           path.Root("test"),
				PathExpression: path.MatchRoot("test"),
				ConfigValue:    tc.value,
			}
			response := validator.StringResponse{}

			NewValidatorOfferURL().ValidateString(context.Background(), request, &response)

			valueStr := "<null or unknown>"
			if !tc.value.IsNull() && !tc.value.IsUnknown() {
				valueStr = tc.value.ValueString()
			}

			if tc.expectError {
				assert.True(t, response.Diagnostics.HasError(), "Expected error for value %s, but got none", valueStr)
			} else {
				assert.False(t, response.Diagnostics.HasError(), "Expected no error for value %s, but got: %v", valueStr, response.Diagnostics)
			}
		})
	}
}
