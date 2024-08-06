// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.ConfigValidator = &RequiresJAASValidator{}
var _ resource.ConfigValidator = &RequiresJAASValidator{}

// RequiresJAASValidator enforces that the resource can only be used with JAAS.
type RequiresJAASValidator struct {
	Client *juju.Client
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v RequiresJAASValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// // MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v RequiresJAASValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that this resource can only be used with JAAS"
}

// ValidateResource performs the validation on the data source.
func (v RequiresJAASValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	resp.Diagnostics = v.Validate(ctx, req.Config)
}

// ValidateResource performs the validation on the resource.
func (v RequiresJAASValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics = v.Validate(ctx, req.Config)
}

// Validate runs the main validation logic of the validator, reading configuration data out of `config` and returning with diagnostics.
func (v RequiresJAASValidator) Validate(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	if v.Client != nil && v.Client.IsJAAS() {
		diags.AddError("Attempted use of resource without JAAS.",
			"This resource can only be used with a JAAS setup offering additional enterprise features - see https://jaas.ai/ for more details.")
	}

	return diags
}
