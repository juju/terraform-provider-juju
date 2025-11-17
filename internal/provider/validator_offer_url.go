// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/juju/juju/core/crossmodel"
)

var _ validator.String = validatorOfferURL{}

type validatorOfferURL struct{}

// NewValidatorOfferURL returns a validator which ensures that the offer URL is valid
// and does not contain a source controller field.
func NewValidatorOfferURL() validator.String {
	return validatorOfferURL{}
}

// Description returns a description of the validator's behavior.
func (v validatorOfferURL) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior.
func (v validatorOfferURL) MarkdownDescription(context.Context) string {
	return "offer URL must be a valid offer URL string and must not include a source controller"
}

// ValidateString performs the validation for offer URL values.
func (v validatorOfferURL) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	// Parse the offer URL using Juju's parser
	parsedURL, err := crossmodel.ParseOfferURL(value)
	if err != nil {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			"offer URL must be a valid offer string",
			value,
		))
		return
	}

	// Check if the source controller field is set
	if parsedURL.Source != "" {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueDiagnostic(
			request.Path,
			"offer URL must not include a source controller. "+
				"Remove the controller prefix from the offer URL (e.g., use 'admin/model.offer' instead of 'controller:admin/model.offer')",
			value,
		))
	}
}
