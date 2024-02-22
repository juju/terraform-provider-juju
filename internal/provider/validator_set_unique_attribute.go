// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/helpers/validatordiag"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

type setNestedIsAttributeUniqueValidator struct {
	PathExpressions path.Expressions
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v setNestedIsAttributeUniqueValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v setNestedIsAttributeUniqueValidator) MarkdownDescription(context.Context) string {
	return fmt.Sprintf("Ensure following attributes have unique values accross the set: %q", v.PathExpressions.String())
}

// Validate runs the main validation logic of the validator, reading configuration data out of `req` and updating `resp` with diagnostics.
func (v setNestedIsAttributeUniqueValidator) ValidateSet(ctx context.Context, req validator.SetRequest, resp *validator.SetResponse) {
	// If the value is unknown or null, there is nothing to validate.
	if req.ConfigValue.IsUnknown() || req.ConfigValue.IsNull() {
		return
	}

	expressions := req.PathExpression.MergeExpressions(v.PathExpressions...)

	for _, expression := range expressions {
		matchedPaths, diags := req.Config.PathMatches(ctx, expression)

		resp.Diagnostics.Append(diags...)

		// Collect all errors
		if diags.HasError() {
			continue
		}
		attrValues := make(map[attr.Value]bool)
		for _, mp := range matchedPaths {
			// If the user specifies the same attribute this validator is applied to,
			// also as part of the input, skip it
			if mp.Equal(req.Path) {
				continue
			}

			var mpVal attr.Value
			diags := req.Config.GetAttribute(ctx, mp, &mpVal)
			resp.Diagnostics.Append(diags...)

			// Collect all errors
			if diags.HasError() {
				continue
			}

			// Delay validation until all involved attribute have a known value
			if mpVal.IsUnknown() {
				return
			}

			if _, ok := attrValues[mpVal]; ok {
				splittedMatchedPath := strings.Split(mp.String(), ".")
				attribute := splittedMatchedPath[len(splittedMatchedPath)-1]
				resp.Diagnostics.Append(validatordiag.InvalidAttributeCombinationDiagnostic(
					req.Path,
					fmt.Sprintf("Set %q has the attribute %q specified multiple times with value %v",
						req.Path,
						attribute,
						mpVal,
					),
				))
			}
			attrValues[mpVal] = true
		}
	}
}
