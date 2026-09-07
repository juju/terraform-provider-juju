// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// InvalidateRevisionIfChannelOrBaseChanges returns a plan modifier that sets the
// revision to Unknown if either the channel or base has changed.
func InvalidateRevisionIfChannelOrBaseChanges() planmodifier.Int64 {
	return &invalidateRevisionModifier{}
}

type invalidateRevisionModifier struct{}

func (m *invalidateRevisionModifier) Description(_ context.Context) string {
	return "If the channel or base changes, the revision must be recalculated unless pinned."
}

func (m *invalidateRevisionModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *invalidateRevisionModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// If the user provided an explicit revision in the config, don't override it.
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() {
		return
	}

	// Check whether the channel or base (sibling attributes) is changing.
	// Because 'charm' is a ListNestedBlock, we look at the first element [0].
	channelPath := req.Path.ParentPath().AtName("channel")
	basePath := req.Path.ParentPath().AtName("base")

	var stateChannel, planChannel, stateBase, planBase types.String
	diags := req.State.GetAttribute(ctx, channelPath, &stateChannel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.Plan.GetAttribute(ctx, channelPath, &planChannel)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.State.GetAttribute(ctx, basePath, &stateBase)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	diags = req.Plan.GetAttribute(ctx, basePath, &planBase)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the channel or base is changing, mark the revision as Unknown (Known After Apply).
	// An unknown value does not indicate a change, so only invalidate the revision when the
	// planned value is known and differs from state.
	if (!planChannel.IsUnknown() && !planChannel.Equal(stateChannel)) ||
		(!planBase.IsUnknown() && !planBase.Equal(stateBase)) {
		resp.PlanValue = types.Int64Unknown()
	}
}
