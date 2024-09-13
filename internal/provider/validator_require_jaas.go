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

var _ datasource.ConfigValidator = &RequiresJAASValidator{}
var _ resource.ConfigValidator = &RequiresJAASValidator{}

// RequiresJAASValidator enforces that the resource can only be used with JAAS.
type RequiresJAASValidator struct {
	client *juju.Client
}

// NewRequiresJAASValidator returns a new validator that enforces a resource can
// only be created against JAAS.
func NewRequiresJAASValidator(client *juju.Client) RequiresJAASValidator {
	return RequiresJAASValidator{
		client: client,
	}
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v RequiresJAASValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v RequiresJAASValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that this resource can only be used with JAAS"
}

// ValidateResource performs the validation on the data source.
func (v RequiresJAASValidator) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate()
}

// ValidateResource performs the validation on the resource.
func (v RequiresJAASValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate()
}

// validate runs the main validation logic of the validator, reading configuration data out of `config` and returning with diagnostics.
func (v RequiresJAASValidator) validate() diag.Diagnostics {
	var diags diag.Diagnostics

	// Return without error if a nil client is detected.
	// This is possible since validation is called at various points throughout resource creation.
	if v.client != nil && !v.client.IsJAAS() {
		diags.AddError("Attempted use of resource without JAAS.",
			"This resource can only be used with JAAS, which offers additional enterprise features - see https://jaas.ai/ for more details.")
	}

	return diags
}
