// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/names/v5"
)

// Note: the "JAAS controller access resource" is slightly different from the other
// JAAS access resources. Controller access refers to direct permission on only the
// JAAS controller. Therefore there is no target object exposed to the user.

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasAccessControllerResource{}
var _ resource.ResourceWithConfigure = &jaasAccessControllerResource{}
var _ resource.ResourceWithImportState = &jaasAccessControllerResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessControllerResource{}

// NewJAASAccessControllerResource returns a new resource for JAAS controller access.
func NewJAASAccessControllerResource() resource.Resource {
	return &jaasAccessControllerResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  controllerInfo{},
		resourceLogName: LogResourceJAASAccessController,
	}}
}

type controllerInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j controllerInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (genericJAASAccessData, names.Tag) {
	controllerAccess := jaasAccessControllerResourceController{}
	diag.Append(getter.Get(ctx, &controllerAccess)...)
	return genericJAASAccessData(controllerAccess), names.NewControllerTag("jimm")
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j controllerInfo) Save(ctx context.Context, setter Setter, info genericJAASAccessData, _ names.Tag) diag.Diagnostics {
	return setter.Set(ctx, jaasAccessControllerResourceController(info))
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j controllerInfo) ImportHint() string {
	return "controller-jimm:<access-level>"
}

type jaasAccessControllerResource struct {
	genericJAASAccessResource
}

type jaasAccessControllerResourceController struct {
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS controller access resource.
func (a *jaasAccessControllerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_controller"
}

// Schema defines the schema for the JAAS controller access resource.
func (a *jaasAccessControllerResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := a.partialAccessSchema()
	// The controller access schema has no target object.
	// The only target is the JAAS controller so we don't need user input.
	schema := schema.Schema{
		Description: "A resource that represents direct access the JAAS controller.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
