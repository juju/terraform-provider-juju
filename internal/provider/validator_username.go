package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/juju/names/v5"
)

var _ validator.String = usernameValidator{}

type usernameValidator struct{}

// UsernameStringIsValid returns an AttributeValidator which ensures that any configured
// username string is valid.
func UsernameStringIsValid() validator.String {
	return usernameValidator{}
}

func (v usernameValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v usernameValidator) MarkdownDescription(context.Context) string {
	return "Ensure value is a valid user name"
}

// ValidateString performs the validation for string values.
func (v usernameValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if !names.IsValidUser(value) {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}
