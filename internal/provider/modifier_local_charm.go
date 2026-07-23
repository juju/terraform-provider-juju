// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// LocalCharmHashModifier returns a plan modifier for the local_charm
// path_hash attribute. It computes the content hash of the archive
// referenced by the sibling path attribute and forces a replacement when the
// charm name in the new archive differs from the deployed one. A content-only
// change produces a diff that drives an in-place refresh.
func LocalCharmHashModifier() planmodifier.String {
	return &localCharmHashModifier{}
}

type localCharmHashModifier struct{}

func (m *localCharmHashModifier) Description(_ context.Context) string {
	return "Computes the content hash of the local charm archive referenced by path and forces a replacement if the charm name changed."
}

func (m *localCharmHashModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *localCharmHashModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// The hash lives in the same local_charm block element as path.
	pathPath := req.Path.ParentPath().AtName("path")

	var localPath types.String
	diags := req.Plan.GetAttribute(ctx, pathPath, &localPath)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if localPath.IsNull() || localPath.IsUnknown() || localPath.ValueString() == "" {
		resp.PlanValue = types.StringNull()
		return
	}

	info, err := juju.ReadLocalCharmInfo(localPath.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Local Charm Error", err.Error())
		return
	}

	resp.PlanValue = types.StringValue(info.Hash)

	// Replacement only applies to updates, and only when the file content
	// actually changed.
	if !req.State.Raw.IsKnown() || req.State.Raw.IsNull() || resp.PlanValue.Equal(req.StateValue) {
		return
	}

	// The content changed. Compare the new charm's metadata name against the
	// name currently recorded in state.
	namePath := req.Path.ParentPath().AtName("name")
	var stateName types.String
	diags = req.State.GetAttribute(ctx, namePath, &stateName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.RequiresReplace = !stateName.IsNull() && stateName.ValueString() != info.Name
}

// OriginHashModifier returns a plan modifier for the local_charm origin_hash
// attribute. The controller-reported hash changes with the deployed charm, so
// plain UseStateForUnknown would trip "inconsistent result after apply".
// Instead it keeps the prior value while the charm is unchanged and becomes
// unknown when a charm-defining attribute changes or on create.
func OriginHashModifier() planmodifier.String {
	return &originHashModifier{}
}

type originHashModifier struct{}

func (m *originHashModifier) Description(_ context.Context) string {
	return "Keeps origin_hash from state unless the charm is changing, in which case it becomes unknown."
}

func (m *originHashModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *originHashModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// On create there is no prior state; leave the value unknown so it is
	// populated from the controller after apply.
	if !req.State.Raw.IsKnown() || req.State.Raw.IsNull() {
		resp.PlanValue = types.StringUnknown()
		return
	}

	// If a charm-defining attribute is planned to change, the deployed charm
	// (and therefore its controller-reported hash) will change too, so the
	// value must be unknown to accept the post-apply hash.
	parent := req.Path.ParentPath()
	charmChanging := false

	baseAttr := parent.AtName("base")
	var planBase, stateBase types.String
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, baseAttr, &planBase)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, baseAttr, &stateBase)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if planBase.IsUnknown() || !planBase.Equal(stateBase) {
		charmChanging = true
	}

	// The local charm content is detected by recomputing the file hash from
	// path, because sibling plan modifiers do not observe each other's
	// planned values within the same apply.
	if !charmChanging &&
		localCharmContentChanges(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics) {
		charmChanging = true
	}
	if resp.Diagnostics.HasError() {
		return
	}

	if charmChanging {
		resp.PlanValue = types.StringUnknown()
		return
	}

	// The charm is unchanged: preserve the prior origin hash so no perpetual
	// diff is produced.
	resp.PlanValue = req.StateValue
}

// localCharmContentChanges reports whether the local charm archive referenced
// by the sibling path attribute has different content than what is recorded
// in state. It recomputes the archive hash and compares it against the
// path_hash stored in state. attrPath is the path of any attribute within the
// same local_charm block element.
func localCharmContentChanges(
	ctx context.Context,
	attrPath path.Path,
	plan tfsdk.Plan,
	state tfsdk.State,
	diags *diag.Diagnostics,
) bool {
	parent := attrPath.ParentPath()

	var localPath types.String
	diags.Append(plan.GetAttribute(ctx, parent.AtName("path"), &localPath)...)
	if diags.HasError() {
		return false
	}
	if localPath.IsNull() || localPath.IsUnknown() || localPath.ValueString() == "" {
		return false
	}

	var stateHash types.String
	diags.Append(state.GetAttribute(ctx, parent.AtName("path_hash"), &stateHash)...)
	if diags.HasError() {
		return false
	}

	info, err := juju.ReadLocalCharmInfo(localPath.ValueString())
	if err != nil {
		diags.AddError("Local Charm Error", err.Error())
		return false
	}

	// A null or empty state hash (for example after drift invalidated it)
	// counts as a content change so dependent computed values are
	// recalculated.
	if stateHash.IsNull() || stateHash.ValueString() == "" {
		return true
	}
	return info.Hash != stateHash.ValueString()
}
