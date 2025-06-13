// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/names/v5"
)

type genericSchema map[string]schema.Attribute

// baseAccessSchema returns a map of schema attributes for a JAAS access resource.
// Access resources should use this schema and add any additional attributes e.g. name or uuid.
func baseAccessSchema() genericSchema {
	return map[string]schema.Attribute{
		"access": schema.StringAttribute{
			Description: "Level of access to grant. Changing this value will replace the Terraform resource. Valid access levels are described at https://canonical-jaas-documentation.readthedocs-hosted.com/latest/howto/manage-permissions/#add-a-permission",
			Required:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"users": schema.SetAttribute{
			Description: "List of users to grant access. A valid user is the user's name or email.",
			Optional:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "email must be a valid Juju username")),
				setvalidator.ValueStringsAre(stringvalidator.RegexMatches(basicEmailValidationRe, "email must contain an @ symbol")),
			},
		},
		"groups": schema.SetAttribute{
			Description: "List of groups to grant access. A valid group ID is the group's UUID.",
			Optional:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(jimmnames.IsValidGroupId, "group ID must be valid")),
			},
		},
		"service_accounts": schema.SetAttribute{
			Description: "List of service accounts to grant access. A valid service account is the service account's name.",
			Optional:    true,
			ElementType: types.StringType,
			// service accounts are treated as users but defined separately
			// for different validation and logic in the provider.
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(
					func(s string) bool {
						// Use EnsureValidServiceAccountId instead of IsValidServiceAccountId
						// because we avoid requiring the user to add @serviceaccount for service accounts
						// and opt to add that in the provide code. EnsureValidServiceAccountId adds the
						// @serviceaccount domain before verifying the string is a valid service account ID.
						_, err := jimmnames.EnsureValidServiceAccountId(s)
						return err == nil
					}, "service account ID must be a valid Juju username")),
				setvalidator.ValueStringsAre(stringvalidator.RegexMatches(avoidAtSymbolRe, "service account should not contain an @ symbol")),
			},
		},
		// ID required for imports
		"id": schema.StringAttribute{
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
	}
}

// WithRoles add roles to the schema
func (gS genericSchema) WithRoles() genericSchema {
	gS["roles"] = schema.SetAttribute{
		Description: "List of roles UUIDs to grant access.",
		Optional:    true,
		ElementType: types.StringType,
		Validators: []validator.Set{
			setvalidator.ValueStringsAre(ValidatorMatchString(jimmnames.IsValidRoleId, "role UUID must be valid")),
		},
	}
	return gS
}
