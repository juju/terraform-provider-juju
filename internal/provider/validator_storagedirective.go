// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	jujustorage "github.com/juju/juju/storage"
)

type stringIsStorageDirectiveValidator struct{}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringIsStorageDirectiveValidator) Description(context.Context) string {
	return "string must conform to a storage directive: <pool>,<count>,<size>"
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v stringIsStorageDirectiveValidator) MarkdownDescription(context.Context) string {
	return "string must conform to a storage directive <pool>,<count>,<size>"
}

// Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
func (v stringIsStorageDirectiveValidator) ValidateMap(ctx context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	// If the value of any element is unknown or null, there is nothing to validate.
	for _, element := range req.ConfigValue.Elements() {
		if element.IsUnknown() || element.IsNull() {
			return
		}
	}

	var storageDirectives map[string]string
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &storageDirectives, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	for label, directive := range storageDirectives {
		_, err := jujustorage.ParseConstraints(directive)
		if err != nil {
			resp.Diagnostics.AddAttributeError(
				req.Path,
				"Invalid Storage Directive",
				fmt.Sprintf("%q fails to parse with error: %s", label, err),
			)
		}
	}
}
