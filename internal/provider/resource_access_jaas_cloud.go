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
var _ resource.Resource = &jaasAccessCloudResource{}
var _ resource.ResourceWithConfigure = &jaasAccessCloudResource{}
var _ resource.ResourceWithImportState = &jaasAccessCloudResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessCloudResource{}

// NewJAASAccessCloudResource returns a new resource for JAAS cloud access.
func NewJAASAccessCloudResource() resource.Resource {
	return &jaasAccessCloudResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  cloudInfo{},
		resourceLogName: LogResourceJAASAccessCloud,
	}}
}

type cloudInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j cloudInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (objectsWithAccess, names.Tag) {
	cloudAccess := jaasAccessCloudResourceCloud{}
	diag.Append(getter.Get(ctx, &cloudAccess)...)
	accessCloud := objectsWithAccess{
		ID:              cloudAccess.ID,
		Users:           cloudAccess.Users,
		Groups:          cloudAccess.Groups,
		Roles:           cloudAccess.Roles,
		ServiceAccounts: cloudAccess.ServiceAccounts,
		Access:          cloudAccess.Access,
	}
	// When importing, the cloud name will be empty
	var tag names.Tag
	if cloudAccess.CloudName.ValueString() != "" {
		tag = names.NewCloudTag(cloudAccess.CloudName.ValueString())
	}
	return accessCloud, tag
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j cloudInfo) Save(ctx context.Context, setter Setter, info objectsWithAccess, tag names.Tag) diag.Diagnostics {
	cloudAccess := jaasAccessCloudResourceCloud{
		CloudName:       basetypes.NewStringValue(tag.Id()),
		ID:              info.ID,
		Users:           info.Users,
		Groups:          info.Groups,
		Roles:           info.Roles,
		ServiceAccounts: info.ServiceAccounts,
		Access:          info.Access,
	}
	return setter.Set(ctx, cloudAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j cloudInfo) ImportHint() string {
	return "<cloud-name>:<access-level>"
}

// TagFromID validates the id to be a valid cloud ID
// and returns a cloud tag.
func (j cloudInfo) TagFromID(id string) (names.Tag, error) {
	if !names.IsValidCloud(id) {
		return nil, errors.New("invalid cloud ID")
	}
	return names.NewCloudTag(id), nil
}

type jaasAccessCloudResource struct {
	genericJAASAccessResource
}

type jaasAccessCloudResourceCloud struct {
	CloudName       types.String `tfsdk:"cloud_name"`
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Roles           types.Set    `tfsdk:"roles"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS cloud access resource.
func (a *jaasAccessCloudResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_cloud"
}

// Schema defines the schema for the JAAS cloud access resource.
func (a *jaasAccessCloudResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := baseAccessSchema()
	attributes = attributes.WithRoles()
	attributes["cloud_name"] = schema.StringAttribute{
		Description: "The name of the cloud for access management. If this is changed the resource will be deleted and a new resource will be created.",
		Required:    true,
		Validators: []validator.String{
			ValidatorMatchString(names.IsValidCloud, "cloud must be a valid name"),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	schema := schema.Schema{
		Description: "A resource that represents access to a cloud when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
