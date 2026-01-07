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
var _ resource.Resource = &jaasAccessGroupResource{}
var _ resource.ResourceWithConfigure = &jaasAccessGroupResource{}
var _ resource.ResourceWithImportState = &jaasAccessGroupResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessGroupResource{}

// NewJAASAccessGroupResource returns a new resource for JAAS group access.
func NewJAASAccessGroupResource() resource.Resource {
	return &jaasAccessGroupResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  groupInfo{},
		resourceLogName: LogResourceJAASAccessGroup,
	}}
}

type groupInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j groupInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (objectsWithAccess, names.Tag) {
	groupAccess := jaasAccessModelResourceGroup{}
	diag.Append(getter.Get(ctx, &groupAccess)...)
	accessGroup := objectsWithAccess{
		ID:              groupAccess.ID,
		Users:           groupAccess.Users,
		Groups:          groupAccess.Groups,
		ServiceAccounts: groupAccess.ServiceAccounts,
		Access:          groupAccess.Access,
	}
	// When importing, the group name will be empty
	var tag names.Tag
	if groupAccess.GroupID.ValueString() != "" {
		tag = jimmnames.NewGroupTag(groupAccess.GroupID.ValueString())
	}
	return accessGroup, tag
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j groupInfo) Save(ctx context.Context, setter Setter, info objectsWithAccess, tag names.Tag) diag.Diagnostics {
	groupAccess := jaasAccessModelResourceGroup{
		GroupID:         basetypes.NewStringValue(tag.Id()),
		ID:              info.ID,
		Users:           info.Users,
		Groups:          info.Groups,
		ServiceAccounts: info.ServiceAccounts,
		Access:          info.Access,
	}
	return setter.Set(ctx, groupAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j groupInfo) ImportHint() string {
	return "<group-uuid>:<access-level>"
}

// TagFromID validates the id to be a valid group ID
// and returns a group tag.
func (j groupInfo) TagFromID(id string) (names.Tag, error) {
	if !jimmnames.IsValidGroupId(id) {
		return nil, errors.New("invalid group ID")
	}
	return jimmnames.NewGroupTag(id), nil
}

type jaasAccessGroupResource struct {
	genericJAASAccessResource
}

type jaasAccessModelResourceGroup struct {
	GroupID         types.String `tfsdk:"group_id"`
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS group access resource.
func (a *jaasAccessGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_group"
}

// ConfigValidators sets validators for the group resource.
func (r *jaasAccessGroupResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		NewResourceRequiresJAASValidator(r.client),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("users"),
			path.MatchRoot("groups"),
			path.MatchRoot("service_accounts"),
		),
	}
}

// Schema defines the schema for the JAAS group access resource.
func (a *jaasAccessGroupResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := baseAccessSchema()
	attributes["group_id"] = schema.StringAttribute{
		Description: "The ID of the group for access management. If this is changed the resource will be deleted and a new resource will be created.",
		Required:    true,
		Validators: []validator.String{
			ValidatorMatchString(jimmnames.IsValidGroupId, "group must be a valid UUID"),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	schema := schema.Schema{
		Description: "A resource that represents access to a group when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
