// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/juju/core/model"
)

// baseApplicationRequiresReplaceIf is a plan modifier that sets the RequiresReplace field if the
// model type is IAAS. The reason is that with CAAS the application can be updated in place.
// With IAAS the application needs to be replaced. To make this decision the model type is needed.
// Since you can't access the juju client in the plan modifiers we've added a computed field `model_type`.
// This is set in the state by means of the `stringplanmodifier.UseStateForUnknown()`, so when we update the base
// is always guaranteed to be set.
func baseApplicationRequiresReplaceIf(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.State.Raw.IsKnown() {
		var modelType types.String
		diags := req.State.GetAttribute(ctx, path.Root("model_type"), &modelType)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
		resp.RequiresReplace = modelType.ValueString() == model.IAAS.String()
	}
}
