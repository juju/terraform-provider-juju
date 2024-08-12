// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.ConfigValidator = &AvoidJAASValidator{}
var _ resource.ConfigValidator = &AvoidJAASValidator{}

// AvoidJAASValidator enforces that the resource is not used with JAAS.
// Useful to direct users to more capable resources.
type AvoidJAASValidator struct {
	Client          *juju.Client
	PreferredObject string
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v AvoidJAASValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v AvoidJAASValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that this resource should not be used with JAAS"
}

// ValidateResource performs the validation on the data source.
func (v AvoidJAASValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate()
}

// ValidateResource performs the validation on the resource.
func (v AvoidJAASValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate()
}

// validate runs the main validation logic of the validator, reading configuration data out of `config` and returning with diagnostics.
func (v AvoidJAASValidator) validate() diag.Diagnostics {
	var diags diag.Diagnostics

	// Return without error if a nil client is detected.
	// This is possible since validation is called at various points throughout resource creation.
	if v.Client != nil && v.Client.IsJAAS() {
		hint := ""
		if v.PreferredObject != "" {
			hint = "Try the " + v.PreferredObject + " resource instead."
		}
		diags.AddError("Invalid use of resource with JAAS.",
			"This resource is not supported with JAAS. "+
				hint+
				"JAAS offers additional enterprise features through the use of dedicated resources. "+
				"See https://jaas.ai/ for more details.")
	}
	return diags
}
