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

var _ resource.Resource = &jaasGroupResource{}
var _ resource.ResourceWithConfigure = &jaasGroupResource{}

type jaasGroupResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// NewJAASGroupResource returns a new instance of the JAAS group resource.
func NewJAASGroupResource() resource.Resource {
	return &jaasGroupResource{}
}

type jaasGroupResourceModel struct {
	Name types.String `tfsdk:"name"`
	UUID types.String `tfsdk:"uuid"`
}

// Metadata returns the metadata for the JAAS group resource.
func (r *jaasGroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_group"
}

// Schema defines the schema for JAAS groups.
func (r *jaasGroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a group in JAAS",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "Name of the group",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(
						names.IsValidGroupName,
						"must start with a letter, end with a letter or number, and contain only letters, numbers, periods, underscores, and hyphens",
					),
				},
			},
			"uuid": schema.StringAttribute{
				Description: "UUID of the group",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure sets up the JAAS group resource with the provider data.
func (resource *jaasGroupResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*juju.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *juju.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	resource.client = client
	// Create the local logging subsystem here, using the TF context when creating it.
	resource.subCtx = tflog.NewSubsystem(ctx, LogResourceJAASGroup)
}

// Create attempts to create the group represented by the resource in JAAS.
func (resource *jaasGroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASGroup, "create")
		return
	}

	// Read Terraform configuration from the request into the model
	var plan jaasGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Add the group to JAAS
	uuid, err := resource.client.Jaas.AddGroup(ctx, plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add group %q, got error: %s", plan.Name.ValueString(), err))
		return
	}

	// Set the UUID in the state
	plan.UUID = types.StringValue(uuid)
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

// Read attempts to read the group represented by the resource from JAAS.
func (resource *jaasGroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASGroup, "read")
		return
	}

	// Read the Terraform state from the request into the model
	var state jaasGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the group from JAAS
	group, err := resource.client.Jaas.ReadGroup(ctx, state.UUID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get group %q, got error: %s", state.Name.ValueString(), err))
		return
	}

	// Set the group name in the state
	state.Name = types.StringValue(group.Name)
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update attempts to rename the group represented by the resource in JAAS.
func (resource *jaasGroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASGroup, "update")
		return
	}

	// Read the current state from the request
	var state jaasGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the plan from the request into the model
	var plan jaasGroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the name has not changed, there is nothing to do
	if state.Name.ValueString() == plan.Name.ValueString() {
		return
	}

	// Rename the group in JAAS
	err := resource.client.Jaas.RenameGroup(ctx, state.Name.ValueString(), plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to rename group %q to %q, got error: %s", state.Name.ValueString(), plan.Name.ValueString(), err))
		return
	}

	// Update the state with the new name
	state.Name = plan.Name
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Delete attempts to remove the group represented by the resource from JAAS.
func (resource *jaasGroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Check first if the client is configured
	if resource.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, LogResourceJAASGroup, "delete")
		return
	}

	// Read the Terraform state from the request into the model
	var state jaasGroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the group from JAAS
	err := resource.client.Jaas.RemoveGroup(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove group %q, got error: %s", state.Name.ValueString(), err))
		return
	}
}
