// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// InvalidateRevisionIfChannelChanges returns a plan modifier that sets the
// revision to Unknown if the channel changes or the local charm content
// (local_path_hash) changes. In either case the controller assigns a new
// revision on the resulting charm refresh, so the prior revision must not be
// locked via UseStateForUnknown, otherwise the planned revision would differ
// from the value read back after apply ("inconsistent result after apply").
func InvalidateRevisionIfChannelChanges() planmodifier.Int64 {
	return &invalidateRevisionModifier{}
}

type invalidateRevisionModifier struct{}

func (m *invalidateRevisionModifier) Description(_ context.Context) string {
	return "If the channel or local charm content changes, the revision must be recalculated unless pinned."
}

func (m *invalidateRevisionModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m *invalidateRevisionModifier) PlanModifyInt64(ctx context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	// If the user provided an explicit revision in the config, don't override it.
	if !req.ConfigValue.IsNull() && !req.ConfigValue.IsUnknown() {
		return
	}

	// We need to check if the channel (a sibling attribute) is changing.
	// Because 'charm' is a ListNestedBlock, we look at the first element [0].
	channelPath := req.Path.ParentPath().AtName("channel")

	var stateChannel, planChannel types.String
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

	// If the channel is changing, mark the revision as Unknown (Known After Apply)
	if !planChannel.Equal(stateChannel) {
		resp.PlanValue = types.Int64Unknown()
		return
	}

	// A local charm refresh also produces a new controller-assigned revision.
	// Detect it by recomputing the local charm hash from local_path and comparing
	// against the hash in state.
	if localCharmContentChanges(ctx, req.Path, req.Plan, req.State, &resp.Diagnostics) {
		resp.PlanValue = types.Int64Unknown()
	}
}
