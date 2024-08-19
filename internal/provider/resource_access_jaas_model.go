// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/names/v5"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasAccessModelResource{}
var _ resource.ResourceWithConfigure = &jaasAccessModelResource{}

// NewJAASAccessModelResource returns a new resource for JAAS model access.
func NewJAASAccessModelResource() resource.Resource {
	return &jaasAccessModelResource{genericJAASAccessResource: genericJAASAccessResource{
		targetInfo:      modelInfo{},
		resourceLogName: LogResourceJAASAccessModel,
	}}
}

type modelInfo struct{}

// Identity implements the [resourceInfo] interface, used to extract the model UUID from the Terraform plan/state.
func (j modelInfo) Identity(ctx context.Context, plan Getter, diag *diag.Diagnostics) string {
	modelAccess := jaasAccessModelResourceModel{}
	diag.Append(plan.Get(ctx, &modelAccess)...)
	return names.NewModelTag(modelAccess.ModelUUID.String()).String()
}

type jaasAccessModelResource struct {
	genericJAASAccessResource
}

type jaasAccessModelResourceModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	genericJAASAccessModel
}

// Metadata returns metadata about the JAAS model access resource.
func (a *jaasAccessModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_model"
}

// Schema defines the schema for the JAAS model access resource.
func (a *jaasAccessModelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := a.partialAccessSchema()
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
