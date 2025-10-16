// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// CommaDelimitedStringModifier returns a string modifier that can be used for strings
// containing comma delimited values. It will compare the planned value to the one
// stored in state and should they contain the same comma delimited values it will
// return the version from state - this way terraform will not attempt to update
// the resource just because the order of items in the list changed.
func CommaDelimitedStringModifier() planmodifier.String {
	return commaDelimitedStringModifier{}
}

// commaDelimitedStringModifier implements the plan modifier.
type commaDelimitedStringModifier struct{}

// Description returns a human-readable description of the plan modifier.
func (m commaDelimitedStringModifier) Description(_ context.Context) string {
	return "Compares the comma delimited values to state and returns the version from state if both contain the same values"
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m commaDelimitedStringModifier) MarkdownDescription(_ context.Context) string {
	return "Compares the comma delimited values to state and returns the version from state if both contain the same values"
}

// PlanModifyString implements the plan modification logic.
func (m commaDelimitedStringModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() {
		return
	}

	if req.PlanValue.IsUnknown() {
		return
	}

	stateValue := NewCustomCommaDelimitedStringValue(req.StateValue.ValueString())
	planValue := NewCustomCommaDelimitedStringValue(req.PlanValue.ValueString())

	equal, diags := stateValue.StringSemanticEquals(ctx, planValue)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	if equal {
		resp.PlanValue = req.StateValue
	}
}
