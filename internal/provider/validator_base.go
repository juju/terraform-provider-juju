// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/juju/juju/core/series"
)

type stringIsBaseValidator struct{}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringIsBaseValidator) Description(context.Context) string {
	return "string must conform to name@channel, e.g. ubuntu@22.04"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringIsBaseValidator) MarkdownDescription(context.Context) string {
	return "string must conform to name@channel, e.g. ubuntu@22.04"
}

// Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
func (v stringIsBaseValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	if _, err := series.ParseBaseFromString(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Base",
			"String must conform to name@channel, e.g. ubuntu@22.04",
		)
		return
	}
}
