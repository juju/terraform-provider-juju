// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &secretResource{}
var _ resource.ResourceWithConfigure = &secretResource{}
var _ resource.ResourceWithImportState = &secretResource{}

func NewSecretResource() resource.Resource {
	return &secretResource{}
}

type secretResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type secretResourceModel struct {
	// Model to which the secret belongs. This attribute is required for all actions.
	Model types.String `tfsdk:"model"`
	// Name of the secret to be updated or removed. This attribute is required for 'update' and 'remove' actions.
	Name types.String `tfsdk:"name"`
	// Value of the secret to be added or updated. This attribute is required for 'add' and 'update' actions.
	// Template: [<key>[#base64]]=<value>[ ...]
	Value types.Map `tfsdk:"value"`
	// SecretId is the ID of the secret to be updated or removed. This attribute is required for 'update' and 'remove' actions.
	SecretId types.String `tfsdk:"secret_id"`
	// Info is the description of the secret. This attribute is optional for all actions.
	Info types.String `tfsdk:"info"`
}

func (s *secretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (s *secretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

func (s *secretResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju secret.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The model in which the secret belongs.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the secret.",
				Optional:    true,
			},
			"value": schema.MapAttribute{
				Description: "The value map of the secret. There can be more than one key-value pair.",
				ElementType: types.StringType,
				Required:    true,
				Sensitive:   true,
			},
			"secret_id": schema.StringAttribute{
				Description: "The ID of the secret.",
				Computed:    true,
			},
			"info": schema.StringAttribute{
				Description: "The description of the secret.",
				Optional:    true,
			},
		},
	}
}

// Configure sets up the Juju client for the secret resource.
func (s *secretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.client = client
	// Create the local logging subsystem here, using the TF context when creating it.
	s.subCtx = tflog.NewSubsystem(ctx, LogResourceSecret)
}

// Create creates a new secret in the Juju model.
func (s *secretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secret", "create")
		return
	}

	var plan secretResourceModel

	// Read Terraform plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s.trace(fmt.Sprintf("creating secret resource %q", plan.Name.ValueString()))

	secretValue := make(map[string]string)
	resp.Diagnostics.Append(plan.Value.ElementsAs(ctx, &secretValue, false)...)

	createSecretOutput, err := s.client.Secrets.CreateSecret(&juju.CreateSecretInput{
		ModelName: plan.Model.ValueString(),
		Name:      plan.Name.ValueString(),
		Value:     secretValue,
		Info:      plan.Info.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add secret, got error: %s", err))
		return
	}

	plan.SecretId = types.StringValue(createSecretOutput.SecretId)

	// Save plan into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	s.trace(fmt.Sprintf("created secret resource %q", plan.SecretId))
}

// Read reads the details of a secret in the Juju model.
func (s *secretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secret", "read")
		return
	}

	var state secretResourceModel

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s.trace(fmt.Sprintf("reading secret resource %q", state.SecretId))

	readSecretOutput, err := s.client.Secrets.ReadSecret(&juju.ReadSecretInput{
		SecretId:  state.SecretId.ValueString(),
		ModelName: state.Model.ValueString(),
		Name:      state.Name.ValueStringPointer(),
		Revision:  nil,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret, got error: %s", err))
		return
	}

	// Save the secret details into the Terraform state
	if !state.Name.IsNull() {
		state.Name = types.StringValue(readSecretOutput.Name)
	}
	if !state.Info.IsNull() {
		state.Info = types.StringValue(readSecretOutput.Info)
	}

	secretValue, errDiag := types.MapValueFrom(ctx, types.StringType, readSecretOutput.Value)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Value = secretValue

	// Save state into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("read secret resource %q", state.SecretId))
}

// Update updates the details of a secret in the Juju model.
func (s *secretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secret", "update")
		return
	}

	var plan, state secretResourceModel

	// Read Terraform plan and state into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s.trace(fmt.Sprintf("updating secret resource %q", state.SecretId))
	s.trace(fmt.Sprintf("Update - current state: %v", state))
	s.trace(fmt.Sprintf("Update - proposed plan: %v", plan))

	var err error
	noChange := true

	var updatedSecretInput juju.UpdateSecretInput

	updatedSecretInput.ModelName = state.Model.ValueString()
	updatedSecretInput.SecretId = state.SecretId.ValueString()

	// Check if the secret name has changed
	if !plan.Name.Equal(state.Name) {
		noChange = false
		state.Name = plan.Name
		updatedSecretInput.Name = plan.Name.ValueStringPointer()
	}

	// Check if the secret value has changed
	if !plan.Value.Equal(state.Value) {
		noChange = false
		resp.Diagnostics.Append(plan.Value.ElementsAs(ctx, &state.Value, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(plan.Value.ElementsAs(ctx, &updatedSecretInput.Value, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Check if the secret info has changed
	if !plan.Info.Equal(state.Info) {
		noChange = false
		state.Info = plan.Info
		updatedSecretInput.Info = plan.Info.ValueStringPointer()
	}

	if noChange {
		return
	}

	err = s.client.Secrets.UpdateSecret(&updatedSecretInput)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update secret, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("updated secret resource %q", state.SecretId))
}

// Delete removes a secret from the Juju model.
func (s *secretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secret", "delete")
		return
	}

	var state secretResourceModel

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	s.trace(fmt.Sprintf("deleting secret resource %q", state.SecretId))

	err := s.client.Secrets.DeleteSecret(&juju.DeleteSecretInput{
		ModelName: state.Model.ValueString(),
		SecretId:  state.SecretId.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete secret, got error: %s", err))
		return
	}

	s.trace(fmt.Sprintf("deleted secret resource %q", state.SecretId))
}

func (s *secretResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(s.subCtx, LogResourceSecret, msg, additionalFields...)
}
