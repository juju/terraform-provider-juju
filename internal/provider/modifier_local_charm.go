// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// LocalCharmHashModifier returns a plan modifier for the local_path_hash
// attribute that computes the content hash of the local charm archive
// referenced by the sibling `local_path` attribute and decides whether a
// hash diff requires replacing the application.
//
// When `local_path` is not set (a Charmhub charm), the hash is set to null.
// When the file content changes, the hash changes, producing a plan diff that
// drives an in-place charm refresh. A replacement (destroy + recreate) is
// forced only when the charm's metadata name in the new local file differs
// from the name recorded in state; an identical name means the existing
// application can be refreshed in place.
func LocalCharmHashModifier() planmodifier.String {
	return &localCharmHashModifier{}
}

type localCharmHashModifier struct{}

func (m *localCharmHashModifier) Description(_ context.Context) string {
	return "Computes the content hash of the local charm archive referenced by local_path and forces a replacement if the charm name changed."
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

	// A different charm name means a fundamentally different charm; the
	// application must be replaced. An identical name means we can refresh
	// the existing application in place.
	resp.RequiresReplace = !stateName.IsNull() && stateName.ValueString() != info.Name
}

// InvalidateChannelIfSwitchingToLocalCharm returns a plan modifier that sets
// the computed channel to Unknown when an application switches from a
// Charmhub charm to a local charm. Local charms have no channel, so keeping
// the prior Charmhub channel from state would cause an inconsistent result
// after apply when Read returns the empty string.
func InvalidateChannelIfSwitchingToLocalCharm() planmodifier.String {
	return &invalidateChannelIfSwitchingToLocalCharmModifier{}
}

type invalidateChannelIfSwitchingToLocalCharmModifier struct{}

func (m *invalidateChannelIfSwitchingToLocalCharmModifier) Description(_ context.Context) string {
	return "If switching from a Charmhub charm to a local charm, the channel becomes unknown because local charms have no channel."
}

func (m *invalidateChannelIfSwitchingToLocalCharmModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *invalidateChannelIfSwitchingToLocalCharmModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// If the user explicitly configured channel, preserve it.
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() {
		return
	}

	planLocal, stateLocal := localCharmPresence(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if planLocal && !stateLocal {
		resp.PlanValue = types.StringUnknown()
	}
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
	if !charmChanging {
		planLocal, stateLocal := localCharmPresence(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if planLocal != stateLocal {
			charmChanging = true
		}
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

// localCharmPresence reports whether the plan and state currently refer to a
// local charm, based on whether the sibling local_path attribute is set.
func localCharmPresence(
	ctx context.Context,
	attrPath path.Path,
	plan tfsdk.Plan,
	state tfsdk.State,
	diags *diag.Diagnostics,
) (planLocal bool, stateLocal bool) {
	parent := attrPath.ParentPath()

	var planLocalPath, stateLocalPath types.String
	diags.Append(plan.GetAttribute(ctx, parent.AtName("local_path"), &planLocalPath)...)
	diags.Append(state.GetAttribute(ctx, parent.AtName("local_path"), &stateLocalPath)...)
	if diags.HasError() {
		return false, false
	}

	planLocal = !planLocalPath.IsNull() && !planLocalPath.IsUnknown() && planLocalPath.ValueString() != ""
	stateLocal = !stateLocalPath.IsNull() && !stateLocalPath.IsUnknown() && stateLocalPath.ValueString() != ""
	return planLocal, stateLocal
}
