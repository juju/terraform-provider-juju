// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/utils"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &sshKeyResource{}
var _ resource.ResourceWithConfigure = &sshKeyResource{}
var _ resource.ResourceWithImportState = &sshKeyResource{}

func NewSSHKeyResource() resource.Resource {
	return &sshKeyResource{}
}

type sshKeyResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type sshKeyResourceModel struct {
	ModelName types.String `tfsdk:"model"`
	Payload   types.String `tfsdk:"payload"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (s *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (s *sshKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.subCtx = tflog.NewSubsystem(ctx, LogResourceSSHKey)
}

func (s *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (s *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Resource representing an SSH key.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model to operate in.",
				Required:    true,
			},
			"payload": schema.StringAttribute{
				Description: "SSH key payload.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (s *sshKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "ssh_key", "create")
		return
	}

	var plan sshKeyResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.Payload.ValueString()
	keyIdentifier := utils.GetKeyIdentifierFromSSHKey(payload)
	if keyIdentifier == "" {
		resp.Diagnostics.AddError("Provider Error", fmt.Sprintf("malformed SSH key : %q", payload))
		return
	}

	modelName := plan.ModelName.ValueString()

	if err := s.client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: modelName,
		Payload:   payload,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ssh_key, got error %s", err))
		return
	}
	s.trace(fmt.Sprintf("created ssh_key for: %q", keyIdentifier))

	plan.ID = types.StringValue(newSSHKeyID(modelName, keyIdentifier))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func newSSHKeyID(modelName string, keyIdentifier string) string {
	return fmt.Sprintf("sshkey:%s:%s", modelName, keyIdentifier)
}

// Keys can be imported with the name of the model and the identifier of the key
// ssh_key:<modelName>:<ssh-key-identifier>
// the key identifier is currently based on the comment section of the ssh key
// (e.g. user@hostname) (TODO: issue #267)
func retrieveModelKeyNameFromID(id string, d *diag.Diagnostics) (string, string) {
	tokens := strings.Split(id, ":")
	//If importing with an incorrect ID we need to catch and provide a user-friendly error
	if len(tokens) != 3 {
		d.AddError("Malformed ID", fmt.Sprintf("unable to parse model name and user from provided ID: %q", id))
		return "", ""
	}
	return tokens[1], tokens[2]
}

func (s *sshKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "ssh_key", "read")
		return
	}

	var plan sshKeyResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, keyIdentifier := retrieveModelKeyNameFromID(plan.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := s.client.SSHKeys.ReadSSHKey(&juju.ReadSSHKeyInput{
		ModelName:     modelName,
		KeyIdentifier: keyIdentifier,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ssh key, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("read ssh key resource %q", plan.ID.ValueString()))

	plan.ModelName = types.StringValue(result.ModelName)
	plan.Payload = types.StringValue(result.Payload)

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (s *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "ssh_key", "update")
		return
	}

	var plan, state sshKeyResourceModel

	// Get the Terraform state from the request into the state model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Read Terraform configuration from the request into the plan model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Return early if nothing has changed
	if plan.Payload.Equal(state.Payload) && plan.ModelName.Equal(state.ModelName) {
		return
	}

	modelName, keyIdentifier := retrieveModelKeyNameFromID(plan.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the key
	if err := s.client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName:     modelName,
		KeyIdentifier: keyIdentifier,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ssh key for updating, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("ssh key deleted : %q", state.ID.ValueString()))

	// Create a new key
	if err := s.client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: plan.ModelName.ValueString(),
		Payload:   plan.Payload.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ssh key for updating, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("ssh key created : %q", plan.ID.ValueString()))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
//
// Juju refers to deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func (s *sshKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "ssh_key", "delete")
		return
	}

	var plan sshKeyResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, keyIdentifier := retrieveModelKeyNameFromID(plan.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the key
	if err := s.client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName:     modelName,
		KeyIdentifier: keyIdentifier,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ssh key during delete, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("delete ssh_key resource : %q", plan.ID.ValueString()))
}

func (s *sshKeyResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(s.subCtx, LogResourceSSHKey, msg, additionalFields...)
}
