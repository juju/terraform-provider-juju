// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ resource.Resource = &jaasRoleResource{}
var _ resource.ResourceWithConfigure = &jaasRoleResource{}

type jaasRoleResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// NewJAASRoleResource returns a new instance of the JAAS role resource.
func NewJAASRoleResource() resource.Resource {
	return &jaasRoleResource{}
}

type jaasRoleResourceModel struct {
	Name types.String `tfsdk:"name"`
	UUID types.String `tfsdk:"uuid"`
}

// Metadata returns the metadata for the JAAS role resource.
func (r *jaasRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_role"
}

// Schema defines the schema for JAAS roles.
func (r *jaasRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a role in JAAS",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "Name of the role",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(
						names.IsValidRoleName,
						"must start with a letter, end with a letter or number, and contain only letters, numbers, periods, underscores, and hyphens",
					),
				},
			},
			"uuid": schema.StringAttribute{
				Description: "UUID of the role",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure sets up the JAAS role resource with the provider data.
func (resource *jaasRoleResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	resource.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	resource.subCtx = tflog.NewSubsystem(ctx, LogResourceJAASRole)
}

// Create attempts to create the role represented by the resource in JAAS.
func (resource *jaasRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASRole, "create")
		return
	}

	// Read Terraform configuration from the request into the model
	var plan jaasRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add the role to JAAS
	uuid, err := resource.client.Jaas.AddRole(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add role %q, got error: %s", plan.Name.ValueString(), err))
		return
	}

	// Set the UUID in the state
	plan.UUID = types.StringValue(uuid)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read attempts to read the role represented by the resource from JAAS.
func (resource *jaasRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASRole, "read")
		return
	}

	// Read the Terraform state from the request into the model
	var state jaasRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the role from JAAS
	role, err := resource.client.Jaas.ReadRoleByUUID(ctx, state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get role %q, got error: %s", state.Name.ValueString(), err))
		return
	}

	// Set the role name in the state
	state.Name = types.StringValue(role.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update attempts to rename the role represented by the resource in JAAS.
func (resource *jaasRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASRole, "update")
		return
	}

	// Read the current state from the request
	var state jaasRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the plan from the request into the model
	var plan jaasRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the name has not changed, there is nothing to do
	if plan.Name.Equal(state.Name) {
		return
	}

	// Rename the role in JAAS
	err := resource.client.Jaas.RenameRole(ctx, state.Name.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to rename role %q to %q, got error: %s", state.Name.ValueString(), plan.Name.ValueString(), err))
		return
	}

	// Update the state with the new name
	state.Name = plan.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete attempts to remove the role represented by the resource from JAAS.
func (resource *jaasRoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASRole, "delete")
		return
	}

	// Read the Terraform state from the request into the model
	var state jaasRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the role from JAAS
	err := resource.client.Jaas.RemoveRole(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove role %q, got error: %s", state.Name.ValueString(), err))
		return
	}
}
