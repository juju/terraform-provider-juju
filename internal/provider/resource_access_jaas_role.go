// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"

	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/juju/names/v5"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasAccessRoleResource{}
var _ resource.ResourceWithConfigure = &jaasAccessRoleResource{}
var _ resource.ResourceWithImportState = &jaasAccessRoleResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessRoleResource{}

// NewJAASAccessRoleResource returns a new resource for JAAS role access.
func NewJAASAccessRoleResource() resource.Resource {
	return &jaasAccessRoleResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  roleInfo{},
		resourceLogName: LogResourceJAASAccessRole,
	}}
}

type roleInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j roleInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (objectsWithAccess, names.Tag) {
	roleAccess := jaasAccessModelResourceRole{}
	diag.Append(getter.Get(ctx, &roleAccess)...)
	accessGroup := objectsWithAccess{
		ID:              roleAccess.ID,
		Users:           roleAccess.Users,
		Groups:          roleAccess.Groups,
		ServiceAccounts: roleAccess.ServiceAccounts,
		Access:          roleAccess.Access,
	}
	// When importing, the role name will be empty
	var tag names.Tag
	if roleAccess.RoleID.ValueString() != "" {
		tag = jimmnames.NewRoleTag(roleAccess.RoleID.ValueString())
	}
	return accessGroup, tag
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j roleInfo) Save(ctx context.Context, setter Setter, info objectsWithAccess, tag names.Tag) diag.Diagnostics {
	roleAccess := jaasAccessModelResourceRole{
		RoleID:          basetypes.NewStringValue(tag.Id()),
		ID:              info.ID,
		Users:           info.Users,
		Groups:          info.Groups,
		ServiceAccounts: info.ServiceAccounts,
		Access:          info.Access,
	}
	return setter.Set(ctx, roleAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j roleInfo) ImportHint() string {
	return "<role-uuid>:<access-level>"
}

// TagFromID validates the id to be a valid role ID
// and returns a role tag.
func (j roleInfo) TagFromID(id string) (names.Tag, error) {
	if !jimmnames.IsValidRoleId(id) {
		return nil, errors.New("invalid role ID")
	}
	return jimmnames.NewRoleTag(id), nil
}

type jaasAccessRoleResource struct {
	genericJAASAccessResource
}

type jaasAccessModelResourceRole struct {
	RoleID          types.String `tfsdk:"role_id"`
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS role access resource.
func (a *jaasAccessRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_role"
}

// ConfigValidators sets validators for the resource.
func (r *jaasAccessRoleResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		NewRequiresJAASValidator(r.client),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("users"),
			path.MatchRoot("groups"),
			path.MatchRoot("service_accounts"),
		),
	}
}

// Schema defines the schema for the JAAS role access resource.
func (a *jaasAccessRoleResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := baseAccessSchema()
	attributes["role_id"] = schema.StringAttribute{
		Description: "The UUID of the role for access management. If this is changed the resource will be deleted and a new resource will be created.",
		Required:    true,
		Validators: []validator.String{
			ValidatorMatchString(jimmnames.IsValidRoleId, "role must be a valid UUID"),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	schema := schema.Schema{
		Description: "A resource that represents access to a role when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
