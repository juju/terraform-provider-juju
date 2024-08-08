package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/juju/names/v5"
)

var _ validator.String = modelIDValidator{}

type modelIDValidator struct{}

// ModelIDIsValid returns an AttributeValidator which ensures that any configured
// model ID is valid.
func ModelIDIsValid() validator.String {
	return modelIDValidator{}
}

func (v modelIDValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v modelIDValidator) MarkdownDescription(context.Context) string {
	return "Ensure value is a valid model ID"
}

// Validate performs the validation.
func (v modelIDValidator) ValidateString(ctx context.Context, request validator.StringRequest, response *validator.StringResponse) {
	if request.ConfigValue.IsNull() || request.ConfigValue.IsUnknown() {
		return
	}

	value := request.ConfigValue.ValueString()

	if !names.IsValidModel(value) {
		response.Diagnostics.Append(validatordiag.InvalidAttributeValueMatchDiagnostic(
			request.Path,
			v.Description(ctx),
			value,
		))
	}
}
