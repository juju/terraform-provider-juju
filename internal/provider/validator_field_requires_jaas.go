// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.ConfigValidator = &FieldsRequireJAAS{}
var _ resource.ConfigValidator = &FieldsRequireJAAS{}

// FieldsRequireJAAS enforces that any specified fields
// must only be set when using JAAS.
type FieldsRequireJAAS struct {
	client      *juju.Client
	expressions path.Expressions
}

// NewFieldsRequireJAASValidator returns a new validator that enforces
// any specified fields must only be set when using JAAS.
func NewFieldsRequireJAASValidator(client *juju.Client, expression ...path.Expression) FieldsRequireJAAS {
	return FieldsRequireJAAS{
		client:      client,
		expressions: expression,
	}
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v FieldsRequireJAAS) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v FieldsRequireJAAS) MarkdownDescription(_ context.Context) string {
	return "Enforces that specified fields on a resource can only be used with JAAS"
}

// ValidateResource performs the validation on the data source.
func (v FieldsRequireJAAS) ValidateDataSource(ctx context.Context, req datasource.ValidateConfigRequest, resp *datasource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate(ctx, req.Config)
}

// ValidateResource performs the validation on the resource.
func (v FieldsRequireJAAS) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics = v.validate(ctx, req.Config)
}

// validate runs the main validation logic of the validator,
// returning true if validation passes.
func (v FieldsRequireJAAS) validate(ctx context.Context, config tfsdk.Config) diag.Diagnostics {
	var configuredPaths path.Paths
	var diags diag.Diagnostics

	// The logic below is mostly copied from Terraform's AtLeastOneOfValidator
	// which takes care to handle Null and Unknown values.
	for _, expression := range v.expressions {
		matchedPaths, matchedPathsDiags := config.PathMatches(ctx, expression)

		diags.Append(matchedPathsDiags...)

		// Collect all errors
		if matchedPathsDiags.HasError() {
			continue
		}

		for _, matchedPath := range matchedPaths {
			var value attr.Value
			getAttributeDiags := config.GetAttribute(ctx, matchedPath, &value)

			diags.Append(getAttributeDiags...)

			// Collect all errors
			if getAttributeDiags.HasError() {
				continue
			}

			// If value is unknown, it may be null or a value, so we cannot
			// know if the validator should succeed or not. Collect the path
			// path so we use it to skip the validation later and continue to
			// collect all path matching diagnostics.
			if value.IsUnknown() {
				continue
			}

			// If value is null, move onto the next one.
			if value.IsNull() {
				continue
			}

			// Value is known and not null, it is configured.
			configuredPaths.Append(matchedPath)
		}
	}

	// If there is a configured value for any specified
	// field, enforce that JAAS is being used.
	if len(configuredPaths) > 0 {
		if v.client != nil && !v.client.IsJAAS() {
			diags.AddError("Attempted use of field without JAAS.",
				fmt.Sprintf("The following field(s) can only be set when using JAAS, "+
					"which offers additional enterprise features - see https://jaas.ai/ for more details.\n"+
					"%s", configuredPaths.String()))
		}
	}

	return diags
}
