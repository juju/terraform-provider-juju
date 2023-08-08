package provider

import (
	"context"
	"fmt"
	"strings"

	frameworkdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkResSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
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

func (s *sshKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
}

func (s *sshKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

func (s *sshKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = frameworkResSchema.Schema{
		Description: "Resource representing an SSH key.",
		Attributes: map[string]frameworkResSchema.Attribute{
			"model": frameworkResSchema.StringAttribute{
				Description: "The name of the model to operate in.",
				Required:    true,
			},
			"payload": frameworkResSchema.StringAttribute{
				Description: "SSH key payload.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": frameworkResSchema.StringAttribute{
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

	var data sshKeyResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelName := data.ModelName.ValueString()
	modelUUID, err := s.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}
	payload := data.Payload.ValueString()
	user := utils.GetUserFromSSHKey(payload)
	if user == "" {
		resp.Diagnostics.AddError("Client Error", "malformed SSH key, user not found")
		return
	}

	err = s.client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		Payload:   payload,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ssh_key, got error %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("create ssh_key with payload: %q", payload))

	data.ID = types.StringValue(newSSHKeyID(modelName, user))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func newSSHKeyID(modelName string, user string) string {
	return fmt.Sprintf("sshkey:%s:%s", modelName, user)
}

func retrieveSSHKeyInfoFromID(id string, d *frameworkdiag.Diagnostics) (string, string) {
	tokens := strings.Split(id, ":")
	//If importing with an incorrect ID we need to catch and provide a user-friendly error
	if len(tokens) != 2 {
		d.AddError("Malformed ID", fmt.Sprintf("unable to parse model name and user from provided ID: %q", id))
		return "", ""
	}
	return tokens[0], tokens[1]
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

	modelName, user := retrieveSSHKeyInfoFromID(plan.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	modelInfo, err := s.client.Models.GetModelByName(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}

	result, err := s.client.SSHKeys.ReadSSHKey(&juju.ReadSSHKeyInput{
		ModelName: modelInfo.Name,
		ModelUUID: modelInfo.UUID,
		User:      user,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read ssh key, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("read ssh key resource %q", plan.ID.ValueString()))

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

	modelName := state.ModelName.ValueString()
	modelUUID, err := s.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}

	payload := state.Payload.ValueString()
	user := utils.GetUserFromSSHKey(payload)
	if user == "" {
		resp.Diagnostics.AddError("Client Error", "malformed SSH key, user not found")
		return
	}

	// Delete the key
	err = s.client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		User:      user,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ssh key for updating, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("ssh key deleted : %q", state.ID.ValueString()))

	// Get the model name from the plan because it might have changed
	newModelName := state.ModelName.ValueString()
	newModelUUID, err := s.client.Models.ResolveModelUUID(newModelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}

	// Create a new key
	err = s.client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: newModelName,
		ModelUUID: newModelUUID,
		Payload:   plan.Payload.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create ssh key for updating, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("ssh key created : %q", plan.ID.ValueString()))

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

	var data sshKeyResourceModel

	// Read Terraform configuration from the request into the data model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName := data.ModelName.ValueString()
	modelUUID, err := s.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}

	payload := data.Payload.ValueString()
	user := utils.GetUserFromSSHKey(payload)
	if user == "" {
		resp.Diagnostics.AddError("Client Error", "malformed SSH key, user not found")
		return
	}
	// Delete the key
	err = s.client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		User:      user,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete ssh key for updating, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("delete ssh_key resource : %q", data.ID.ValueString()))
}
