// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/juju/core/model"
)

// charmBlockRequiresReplace returns a plan modifier for the `charm` and
// `local_charm` blocks that decides application replacement by comparing the
// effective charm across both blocks between state and plan.
//
// It is a list-level modifier on both blocks rather than a per-attribute one.
// A list modifier runs when its block is added or removed, so it reliably
// observes a switch between the two blocks. A per-attribute modifier on the
// removed block may not run at all.
//
// Replacement rules:
//   - The charm name changed (a rename is not in-place).
//   - The base changed on an IAAS model (CAAS updates in place, IAAS does not).
//
// Switching charm source with the same name and base is an in-place Update, so
// it must not replace.
func charmBlockRequiresReplace() planmodifier.List {
	return charmBlockReplaceModifier{}
}

type charmBlockReplaceModifier struct{}

func (m charmBlockReplaceModifier) Description(_ context.Context) string {
	return "Replaces the application when the effective charm name changes, " +
		"or when the base changes on an IAAS model, but not when merely " +
		"switching between the charm and local_charm blocks."
}

func (m charmBlockReplaceModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m charmBlockReplaceModifier) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// Nothing to do on create (no prior state) or destroy (no plan).
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var stateCharm, stateLocal, planCharm, planLocal types.List
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(CharmKey), &stateCharm)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(LocalCharmKey), &stateLocal)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(CharmKey), &planCharm)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, path.Root(LocalCharmKey), &planLocal)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateEff, diags := resolveCharm(ctx, stateCharm, stateLocal)
	resp.Diagnostics.Append(diags...)
	planEff, diags := resolveCharm(ctx, planCharm, planLocal)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A rename always forces replacement.
	if stateEff.Name != "" && planEff.Name != "" && stateEff.Name != planEff.Name {
		resp.RequiresReplace = true
		return
	}

	// A base change forces replacement only on IAAS models.
	// Skip when the base is unknown, absent, or unchanged.
	stateBase := stateEff.Base
	planBase := planEff.Base
	if stateBase.IsNull() || stateBase.IsUnknown() ||
		planBase.IsNull() || planBase.IsUnknown() ||
		stateBase.Equal(planBase) {
		return
	}

	var modelType types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("model_type"), &modelType)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.RequiresReplace = modelType.ValueString() == model.IAAS.String()
}
