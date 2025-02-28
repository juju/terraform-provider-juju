// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/diag"
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
var _ resource.Resource = &jaasAccessModelResource{}
var _ resource.ResourceWithConfigure = &jaasAccessModelResource{}
var _ resource.ResourceWithImportState = &jaasAccessModelResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessModelResource{}

// NewJAASAccessModelResource returns a new resource for JAAS model access.
func NewJAASAccessModelResource() resource.Resource {
	return &jaasAccessModelResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  modelInfo{},
		resourceLogName: LogResourceJAASAccessModel,
	}}
}

type modelInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j modelInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (objectsWithAccess, names.Tag) {
	modelAccess := jaasAccessModelResourceModel{}
	diag.Append(getter.Get(ctx, &modelAccess)...)
	accessModel := objectsWithAccess{
		ID:              modelAccess.ID,
		Users:           modelAccess.Users,
		Groups:          modelAccess.Groups,
		Roles:           modelAccess.Roles,
		ServiceAccounts: modelAccess.ServiceAccounts,
		Access:          modelAccess.Access,
	}
	return accessModel, names.NewModelTag(modelAccess.ModelUUID.ValueString())
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j modelInfo) Save(ctx context.Context, setter Setter, info objectsWithAccess, tag names.Tag) diag.Diagnostics {
	modelAccess := jaasAccessModelResourceModel{
		ModelUUID:       basetypes.NewStringValue(tag.Id()),
		ID:              info.ID,
		Users:           info.Users,
		Groups:          info.Groups,
		Roles:           info.Roles,
		ServiceAccounts: info.ServiceAccounts,
		Access:          info.Access,
	}
	return setter.Set(ctx, modelAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j modelInfo) ImportHint() string {
	return "<model-UUID>:<access-level>"
}

// TagFromID validates the id to be a valid model ID
// and returns a model tag.
func (j modelInfo) TagFromID(id string) (names.Tag, error) {
	if !names.IsValidModelName(id) {
		return nil, errors.New("invalid model ID")
	}
	return names.NewModelTag(id), nil
}

type jaasAccessModelResource struct {
	genericJAASAccessResource
}

type jaasAccessModelResourceModel struct {
	ModelUUID       types.String `tfsdk:"model_uuid"`
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Roles           types.Set    `tfsdk:"roles"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS model access resource.
func (a *jaasAccessModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_model"
}

// Schema defines the schema for the JAAS model access resource.
func (a *jaasAccessModelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := baseAccessSchema()
	attributes = attributes.WithRoles()
	attributes["model_uuid"] = schema.StringAttribute{
		Description: "The uuid of the model for access management. If this is changed the resource will be deleted and a new resource will be created.",
		Required:    true,
		Validators: []validator.String{
			ValidatorMatchString(names.IsValidModel, "model must be a valid UUID"),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	schema := schema.Schema{
		Description: "A resource that represent access to a model when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
