// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

var _ planmodifier.Object = requireReplaceIfNotConfiguredModifier{}

// requireReplaceIfNotConfiguredModifier is a plan modifier that requires resource replacement
// when the block is removed from the configuration.
type requireReplaceIfNotConfiguredModifier struct{}

// RequireReplaceIfNotConfigured returns a plan modifier that requires resource replacement
// when the block is removed from the configuration.
func RequireReplaceIfNotConfigured() planmodifier.Object {
	return requireReplaceIfNotConfiguredModifier{}
}

func (m requireReplaceIfNotConfiguredModifier) Description(ctx context.Context) string {
	return "Requires resource replacement when the block is removed after being configured."
}

func (m requireReplaceIfNotConfiguredModifier) MarkdownDescription(ctx context.Context) string {
	return "Requires resource replacement when the block is removed after being configured."
}

func (m requireReplaceIfNotConfiguredModifier) PlanModifyObject(ctx context.Context, req planmodifier.ObjectRequest, resp *planmodifier.ObjectResponse) {
	// If the state is null, this is a create operation, no need to check
	if req.StateValue.IsNull() {
		return
	}

	// If the plan is null or unknown, this might be a destroy or we can't determine yet
	if req.PlanValue.IsNull() && !req.StateValue.IsNull() {
		// The block is being removed - require replacement
		resp.RequiresReplace = true
		resp.Diagnostics.AddAttributeWarning(
			req.Path,
			"Configuration Removal Requires Replacement",
			"Removing the configuration block requires resource replacement. "+
				"The controller will be destroyed and recreated without the removed configuration.",
		)
	}
}
