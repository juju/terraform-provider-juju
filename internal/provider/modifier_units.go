// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func UnitCountModifier() planmodifier.Int64 {
	return unitCountModifier{}
}

// unitCountModifier implements the plan modifier.
type unitCountModifier struct{}

// Description returns a human-readable description of the plan modifier.
func (m unitCountModifier) Description(_ context.Context) string {
	return "Sets the number of units to the number of machines (if specified)."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m unitCountModifier) MarkdownDescription(_ context.Context) string {
	return "Sets the number of units to the number of machines (if specified)."
}

// PlanModifyBool implements the plan modification logic.
func (m unitCountModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// Do nothing if there is a known configuration value.
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() {
		return
	}

	var machines basetypes.SetValue
	diags := req.Plan.GetAttribute(ctx, path.Root("machines"), &machines)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	if len(machines.Elements()) > 0 {
		resp.PlanValue = types.Int64Value(int64(len(machines.Elements())))
		return
	}

	resp.PlanValue = types.Int64Value(1)
}
