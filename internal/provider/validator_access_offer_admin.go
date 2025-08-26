// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ resource.ConfigValidator = &AvoidJAASValidator{}

// AdminOfferUserValidator enforces the user specified in the provider is also
// present in the admin set of the juju_access_offer.
type AdminOfferUserValidator struct {
	client *juju.Client
}

// NewAdminOfferUserValidator creates a new validator that enforces the user
// specified in the provider is also present in the admin set of the juju_access_offer.
func NewAdminOfferUserValidator(client *juju.Client) AdminOfferUserValidator {
	return AdminOfferUserValidator{
		client: client,
	}
}

// Description returns a plain text description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v AdminOfferUserValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription returns a markdown formatted description of the validator's behavior, suitable for a practitioner to understand its impact.
func (v AdminOfferUserValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that the admin set includes the identity configured in the provider."
}

// ValidateResource performs the validation on the resource.
func (v AdminOfferUserValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var diags diag.Diagnostics

	var adminSet types.Set
	diags = req.Config.GetAttribute(ctx, path.Root("admin"), &adminSet)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	if adminSet.IsUnknown() || adminSet.IsNull() {
		diags.AddAttributeError(
			path.Root("admin"),
			"List of administrators does not include the identity configured in the provider",
			"The identity configured in the provider must be included in the admin set.\n\n"+
				"Ensure that the provider identity is included in the admin set.\n",
		)
		return
	}

	var admins []string
	diags = adminSet.ElementsAs(ctx, &admins, true)
	if diags.HasError() {
		resp.Diagnostics = diags
		return
	}

	// Return without error if a nil client is detected.
	// This is possible since validation is called at various points throughout resource creation.
	if v.client != nil {
		providerUser := v.client.Username()

		found := false
		for _, admin := range admins {
			if admin == providerUser {
				found = true
				break
			}
		}
		if !found {
			diags.AddAttributeError(
				path.Root("admin"),
				"List of administrators does not include the identity configured in the provider",
				"The identity configured in the provider must be included in the admin set.\n\n"+
					"Ensure that the user '"+providerUser+"' is included in the admin set.\n",
			)
			resp.Diagnostics = diags
			return
		}
	}
}
