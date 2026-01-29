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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/utils/v3/ssh"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &sshKeyResource{}
var _ resource.ResourceWithConfigure = &sshKeyResource{}
var _ resource.ResourceWithImportState = &sshKeyResource{}
var _ resource.ResourceWithUpgradeState = &sshKeyResource{}

func NewSSHKeyResource() resource.Resource {
	return &sshKeyResource{}
}

type sshKeyResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type sshKeyResourceModel struct {
	Payload types.String `tfsdk:"payload"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type sshKeyResourceModelV0 struct {
	sshKeyResourceModel
	ModelName types.String `tfsdk:"model"`
}

type sshKeyResourceModelV1 struct {
	sshKeyResourceModel
	ModelUUID types.String `tfsdk:"model_uuid"`
}

func (s *sshKeyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (s *sshKeyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	s.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	s.subCtx = tflog.NewSubsystem(ctx, LogResourceSSHKey)
}

func (s *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (s *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     1,
		Description: "Resource representing an SSH key.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model to operate in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
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

	var plan sshKeyResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.Payload.ValueString()
	// Remove trailing newline if present, as it is not part of the key.
	// For example, the key contains a \n when it's read via `file(~/.ssh/id_rsa.pub)`
	// in the terraform configuration.
	payload = strings.TrimSuffix(payload, "\n")
	fingerprint, _, err := ssh.KeyFingerprint(payload)
	if err != nil {
		resp.Diagnostics.AddError("Malformed SSH Key", fmt.Sprintf("Unable to parse SSH key payload: %s", err))
		return
	}

	modelUUID := plan.ModelUUID.ValueString()

	if err := s.client.SSHKeys.CreateSSHKey(ctx, &juju.CreateSSHKeyInput{
		Username:  s.client.Username(),
		ModelUUID: plan.ModelUUID.ValueString(),
		Payload:   payload,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ssh_key, got error %s", err))
		return
	}
	s.trace(fmt.Sprintf("created ssh_key for: %q", fingerprint))

	plan.ID = types.StringValue(newSSHKeyID(modelUUID, fingerprint))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func newSSHKeyID(modelUUID string, keyIdentifier string) string {
	return fmt.Sprintf("sshkey:%s:%s", modelUUID, keyIdentifier)
}

// Keys can be imported with the name of the model and the identifier of the key
// ssh_key:<modelUUID>:<ssh-key-identifier>
// the key identifier is currently based on the comment section of the ssh key
// (e.g. user@hostname) (TODO: issue #267)
func retrieveModelKeyNameFromID(id string, d *diag.Diagnostics) (string, string) {
	tokens := strings.SplitN(id, ":", 3)
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

	var state sshKeyResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID, identifier := retrieveModelKeyNameFromID(state.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := s.client.SSHKeys.ReadSSHKey(ctx, &juju.ReadSSHKeyInput{
		Username:      s.client.Username(),
		ModelUUID:     modelUUID,
		KeyIdentifier: identifier,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ssh key, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("read ssh key resource %q", state.ID.ValueString()))

	state.ModelUUID = types.StringValue(modelUUID)
	state.Payload = types.StringValue(result.Payload)

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (s *sshKeyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// This should never be called, because all of the fields have a `RequiresReplace` plan modifier.
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

	var state sshKeyResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, identifier := retrieveModelKeyNameFromID(state.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the key
	if err := s.client.SSHKeys.DeleteSSHKey(ctx, &juju.DeleteSSHKeyInput{
		Username:      s.client.Username(),
		ModelUUID:     modelName,
		KeyIdentifier: identifier,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ssh key during delete, got error: %s", err))
		return
	}
	s.trace(fmt.Sprintf("delete ssh_key resource : %q", state.ID.ValueString()))
}

// UpgradeState upgrades the state of the sshKey resource.
// This is used to handle changes in the resource schema between versions.
// V0->V1: Convert attribute `model` to `model_uuid`.
func (s *sshKeyResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: sshKeyV0Schema(),
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				sshKeyV0 := sshKeyResourceModelV0{}
				resp.Diagnostics.Append(req.State.Get(ctx, &sshKeyV0)...)

				if resp.Diagnostics.HasError() {
					return
				}

				modelUUID, err := s.client.Models.ModelUUID(sshKeyV0.ModelName.ValueString(), "")
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model UUID for model %q, got error: %s", sshKeyV0.ModelName.ValueString(), err))
					return
				}

				newID := strings.Replace(sshKeyV0.ID.ValueString(), sshKeyV0.ModelName.ValueString(), modelUUID, 1)

				upgradedStateData := sshKeyResourceModelV1{
					ModelUUID: types.StringValue(modelUUID),
					sshKeyResourceModel: sshKeyResourceModel{
						Payload: sshKeyV0.Payload,
						ID:      types.StringValue(newID),
					},
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
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

func sshKeyV0Schema() *schema.Schema {
	return &schema.Schema{
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
