// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.ConfigValidator = &AvoidJAASValidator{}
var _ provider.ConfigValidator = &AvoidJAASValidator{}
var _ resource.ConfigValidator = &AvoidJAASValidator{}

// AvoidJAASValidator enforces that the resource is not used with JAAS.
// Useful to direct users to more capable resources.
type AvoidJAASValidator struct {
	Client          *juju.Client
	PreferredObject string
}

func (v AvoidJAASValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

func (v AvoidJAASValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that this resource should not be used with JAAS"
}

func (v AvoidJAASValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	resp.Diagnostics = v.Validate(ctx, req.Config)
}

func (v AvoidJAASValidator) ValidateProvider(ctx context.Context, req provider.ValidateConfigRequest, resp *provider.ValidateConfigResponse) {
	resp.Diagnostics = v.Validate(ctx, req.Config)
}

func (v AvoidJAASValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics = v.Validate(ctx, req.Config)
}

func (v AvoidJAASValidator) Validate(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	if v.Client != nil {
		if v.Client.IsJAAS() {
			hint := ""
			if v.PreferredObject != "" {
				hint = "Try the " + v.PreferredObject + " resource instead."
			}
			diags.AddWarning("Invalid use of resource with JAAS.",
				"It is not supported to use this resource with a JAAS setup. "+
					hint+
					"JAAS offers additional enterprise features through the use of dedicated resources. "+
					"See the provider documentation for more details.")
		}
	}
	return diags
}
