// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type StringIsResourceKeyValidator struct{}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v StringIsResourceKeyValidator) Description(context.Context) string {
	return "string must conform to a charm resource: a resource revision number from CharmHub or a custom OCI image resource"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v StringIsResourceKeyValidator) MarkdownDescription(context.Context) string {
	return resourceKeyMarkdownDescription
}

// ValidateMap Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
func (v StringIsResourceKeyValidator) ValidateMap(ctx context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	var resourceKey map[string]string
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &resourceKey, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for name, value := range resourceKey {
		if isInt(value) {
			providedRev, err := strconv.Atoi(value)
			if err != nil {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Resource revision",
					fmt.Sprintf("value of %q should be a valid revision number or image URL: %s", name, err),
				)
			}
			if providedRev <= 0 {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid Resource revision",
					fmt.Sprintf("value of %q should be a valid revision number or image URL: %s", name, "Negative revision number is invalid."),
				)
			}
		} else {
			imageUrlPattern := `(?:[a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9]):[\w][\w.-]{0,127}`
			urlRegex := regexp.MustCompile(imageUrlPattern)
			if urlRegex.MatchString(value) {
				return
			} else {
				resp.Diagnostics.AddAttributeError(
					req.Path,
					"Invalid image URL",
					fmt.Sprintf("value of %q should be a valid revision number or image URL: %s", name, "The value format is invalid as a revision number or for an image URL."),
				)
				return
			}
		}
	}
}
