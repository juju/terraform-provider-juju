// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

var _ validator.String = validatorMatchString{}

type validatorMatchString struct {
	isValid func(string) bool
	message string
}

// ValidatorMatchString returns an AttributeValidator which ensures that any configured
// field is valid according to the validation function passed in.
//
// message will be used when printing error messages for invalid values.
func ValidatorMatchString(isValid func(string) bool, message string) validator.String {
	return validatorMatchString{
		isValid: isValid,
		message: message,
	}
}

func (v validatorMatchString) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v validatorMatchString) MarkdownDescription(context.Context) string {
	if v.message != "" {
		return v.message
	}
	return "value must match the provided validation function"
}

// ValidateString performs the validation for string values.
func (v validatorMatchString) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if !v.isValid(value) {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}
