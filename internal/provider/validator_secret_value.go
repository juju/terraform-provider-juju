// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// SecretValueMapValidator rejects null elements in a secret value map (value or
// value_wo) at plan time, giving early feedback rather than the opaque framework
// error that a null string element would otherwise produce. A null element in a
// secret almost always indicates an accidental unset value.
type SecretValueMapValidator struct{}

// Description returns a plain text description of the validator's behavior.
func (v SecretValueMapValidator) Description(context.Context) string {
	return "secret map values must not be null"
}

// MarkdownDescription returns a markdown description of the validator's behavior.
func (v SecretValueMapValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

// ValidateMap runs the validation logic, reading the map from req and updating
// resp with diagnostics.
func (v SecretValueMapValidator) ValidateMap(_ context.Context, req validator.MapRequest, resp *validator.MapResponse) {
	resp.Diagnostics.Append(validateSecretValueMap(req.ConfigValue, req.Path)...)
}

// validateSecretValueMap rejects known null elements of a secret value map.
// Unknown elements (e.g. a value_wo element fed by an ephemeral variable that is
// only resolved at apply) are skipped: they cannot be validated at plan time and
// are re-checked at apply by resolveSecretValue/Update once resolved.
func validateSecretValueMap(value types.Map, basePath path.Path) diag.Diagnostics {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return diags
	}
	for k, elem := range value.Elements() {
		strVal, ok := elem.(types.String)
		if !ok || strVal.IsUnknown() {
			continue
		}
		if strVal.IsNull() {
			diags.AddAttributeError(
				basePath.AtMapKey(k),
				"Null Secret Value",
				fmt.Sprintf("The secret value for key %q must not be null. Remove the"+
					" key or provide a non-null value.", k),
			)
		}
	}
	return diags
}
