// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// ipAddressConditionValidator validates that each element of
// wait_for_ip_addresses is one of the supported aliases or a valid CIDR.
type ipAddressConditionValidator struct{}

// Description returns a plain text description of the validator's behavior.
func (ipAddressConditionValidator) Description(_ context.Context) string {
	return "value must be one of \"public\", \"private\", \"any\" or a valid CIDR (e.g. \"10.0.10.0/24\")"
}

// MarkdownDescription returns a markdown description of the validator's behavior.
func (ipAddressConditionValidator) MarkdownDescription(ctx context.Context) string {
	return ipAddressConditionValidator{}.Description(ctx)
}

// ValidateString performs the validation.
func (v ipAddressConditionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if !juju.ValidIPAddressCondition(req.ConfigValue.ValueString()) {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid IP Address Condition",
			fmt.Sprintf("Value %q is invalid. %s", req.ConfigValue.ValueString(), v.Description(ctx)),
		)
	}
}
