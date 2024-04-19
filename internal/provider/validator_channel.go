// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/juju/charm/v11"
)

type StringIsChannelValidator struct{}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v StringIsChannelValidator) Description(context.Context) string {
	return "string must conform to track/risk or track/risk/branch e.g. latest/stable"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v StringIsChannelValidator) MarkdownDescription(context.Context) string {
	return "string must conform to track/risk or track/risk/branch e.g. latest/stable"
}

// Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
func (v StringIsChannelValidator) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	if channel, err := charm.ParseChannel(req.ConfigValue.ValueString()); err != nil {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Channel",
			err.Error(),
		)
		return
	} else if channel.Track == "" || channel.Risk == "" {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Channel",
			"String must conform to track/risk or track/risk/branch, e.g. latest/stable",
		)
		return
	}
}
