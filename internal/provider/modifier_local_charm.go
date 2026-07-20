// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// LocalCharmHashModifier returns a plan modifier that computes the content
// hash of the local charm archive referenced by the sibling `local_path`
// attribute. When `local_path` is not set (a Charmhub charm), the hash is set
// to null. When the file content changes, the hash changes, producing a plan
// diff that drives an in-place charm refresh (or a replacement if the charm
// name has changed, see LocalCharmRequiresReplace).
func LocalCharmHashModifier() planmodifier.String {
	return &localCharmHashModifier{}
}

type localCharmHashModifier struct{}

func (m *localCharmHashModifier) Description(_ context.Context) string {
	return "Computes the content hash of the local charm archive referenced by local_path."
}

func (m *localCharmHashModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *localCharmHashModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// The hash lives in the same charm block element as local_path.
	localPathPath := req.Path.ParentPath().AtName("local_path")

	var localPath types.String
	diags := req.Plan.GetAttribute(ctx, localPathPath, &localPath)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// No local charm: this is a Charmhub charm, so there is no hash.
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
}

// OriginHashModifier returns a plan modifier for the computed `origin_hash`.
// The controller-reported hash changes with the deployed charm, so plain
// UseStateForUnknown would trip "inconsistent result after apply".
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
	//
	// The base comes from config and can be compared directly. The local charm
	// content is detected by recomputing the file hash from local_path, because
	// sibling plan modifiers do not observe each other's planned values within
	// the same apply.
	parent := req.Path.ParentPath()
	charmChanging := false
	baseAttr := parent.AtName("base")
	var planBase, stateBase attr.Value
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, baseAttr, &planBase)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, baseAttr, &stateBase)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if planBase.IsUnknown() || !planBase.Equal(stateBase) {
		charmChanging = true
	}
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

	// The charm is unchanged: preserve the prior origin hash so drift that
	// was reconciled stays stable and no perpetual diff is produced.
	resp.PlanValue = req.StateValue
}

// localCharmContentChanges reports whether the local charm archive referenced
// by the sibling local_path attribute has different content than what is
// recorded in state. It recomputes the archive hash from local_path and
// compares it against the local_path_hash stored in state. It returns false
// when there is no local charm (a Charmhub charm) or when the content is
// unchanged. attrPath is the path of any attribute within the same charm
// block element; the function derives the sibling paths from its parent.
func localCharmContentChanges(
	ctx context.Context,
	attrPath path.Path,
	plan tfsdk.Plan,
	state tfsdk.State,
	diags *diag.Diagnostics,
) bool {
	parent := attrPath.ParentPath()

	var localPath types.String
	diags.Append(plan.GetAttribute(ctx, parent.AtName("local_path"), &localPath)...)
	if diags.HasError() {
		return false
	}
	// Not a local charm: no local content to compare.
	if localPath.IsNull() || localPath.IsUnknown() || localPath.ValueString() == "" {
		return false
	}

	var stateHash types.String
	diags.Append(state.GetAttribute(ctx, parent.AtName("local_path_hash"), &stateHash)...)
	if diags.HasError() {
		return false
	}

	info, err := juju.ReadLocalCharmInfo(localPath.ValueString())
	if err != nil {
		diags.AddError("Local Charm Error", err.Error())
		return false
	}

	// A null or empty state hash (for example after drift invalidated it, or
	// on first read) counts as a content change so the dependent computed
	// values are recalculated.
	if stateHash.IsNull() || stateHash.ValueString() == "" {
		return true
	}
	return info.Hash != stateHash.ValueString()
}

// LocalCharmRequiresReplace returns a RequiresReplaceIf function for the
// local_path_hash attribute. It forces a replacement (destroy + recreate) only
// when the charm's metadata name in the new local file differs from the name
// recorded in state. When only the content changes but the charm name is the
// same, no replacement is required and the change is applied in place via a
// charm refresh during Update.
func LocalCharmRequiresReplace(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	// Nothing in state yet (create) or no local charm planned: never
	// replace based on the hash.
	if !req.State.Raw.IsKnown() || req.State.Raw.IsNull() {
		return
	}

	localPathPath := req.Path.ParentPath().AtName("local_path")
	var localPath types.String
	diags := req.Plan.GetAttribute(ctx, localPathPath, &localPath)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if localPath.IsNull() || localPath.IsUnknown() || localPath.ValueString() == "" {
		return
	}

	// If the hash has not changed, the file content is identical: nothing
	// to do, and certainly no replacement.
	if req.PlanValue.Equal(req.StateValue) {
		return
	}

	// The content changed. Parse the new charm to compare its metadata name
	// against the name currently recorded in state.
	namePath := req.Path.ParentPath().AtName("name")
	var stateName types.String
	diags = req.State.GetAttribute(ctx, namePath, &stateName)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	info, err := juju.ReadLocalCharmInfo(localPath.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Local Charm Error", err.Error())
		return
	}

	// A different charm name means a fundamentally different charm; the
	// application must be replaced. An identical name means we can refresh
	// the existing application in place.
	resp.RequiresReplace = !stateName.IsNull() && stateName.ValueString() != info.Name
}
