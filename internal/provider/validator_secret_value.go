// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// validateSecretValueMap rejects known null elements of a secret value map.
// Unknown elements are skipped because they cannot be validated during planning.
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
