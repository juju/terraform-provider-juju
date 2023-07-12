package provider

import (
	"context"
	"fmt"
	"strings"

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
var _ resource.Resource = &userResource{}
var _ resource.ResourceWithConfigure = &userResource{}
var _ resource.ResourceWithImportState = &userResource{}

func NewUserResource() resource.Resource {
	return &userResource{}
}

// userResource defines the resource implementation.
type userResource struct {
	client *juju.Client
}

// userResourceModel describes the user resource data model.
// tfsdk must match user resource schema attribute names.
type userResourceModel struct {
	Name        types.String `tfsdk:"name"`
	DisplayName types.String `tfsdk:"display_name"`
	Password    types.String `tfsdk:"password"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (r *userResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (r *userResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// The User resource maps to a juju user that is operated via
	// `juju add-user`, `juju remove-user`
	// Display name is optional.
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represents a Juju User.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name to be assigned to the user",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				Description: "The display name to be assigned to the user (optional)",
				Optional:    true,
			},
			"password": schema.StringAttribute{
				Description: "The password to be assigned to the user",
				Required:    true,
				Sensitive:   true,
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

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (r *userResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

// Create is called when the provider must create a new resource. Config
// and planned state values should be read from the
// CreateRequest and new state values set on the CreateResponse.
func (r *userResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data userResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Users.CreateUser(juju.CreateUserInput{
		Name:        data.Name.ValueString(),
		DisplayName: data.DisplayName.ValueString(),
		Password:    data.Password.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create user, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("created user resource %q", data.Name))

	// Save data into Terraform state
	data.ID = types.StringValue(newIDFromUserName(data.Name.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
// Take the juju api input from the ID, it may not exist in the plan.
// Only set optional values if they exist.
func (r *userResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data userResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	response, err := r.client.Users.ReadUser(userNameFromID(data.ID.ValueString()))
	if err != nil {
		// TODO (hmlanigan) 2023-06-14
		// Add a user NotFound error type to the client.
		// On read, if NotFound, remove the resource:
		// resp.State.RemoveResource()
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read user, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("read user resource %q", data.Name.ValueString()))

	// Save updated data into Terraform state
	plan := userResourceModel{
		Name:     types.StringValue(response.UserInfo.Username),
		Password: data.Password,
		ID:       types.StringValue(fmt.Sprintf("user:%s", response.UserInfo.Username)),
	}
	// Display name is optional, therefore if it doesn't exist in the plan
	// if not set, do not add an empty string as they are not the same thing.
	// Conversely, if the returned user info does not contain an empty string,
	// make it of type ValueStateNull to indicate not set.
	if !data.DisplayName.IsNull() && !data.DisplayName.IsUnknown() && response.UserInfo.DisplayName != "" {
		plan.DisplayName = types.StringValue(response.UserInfo.DisplayName)
	} else if response.UserInfo.DisplayName == "" {
		plan.DisplayName = types.StringNull()
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Update is called to update the state of the resource. Config, planned
// state, and prior state values should be read from the
// UpdateRequest and new state values set on the UpdateResponse.
func (r *userResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data userResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update user can only change the password.
	if err := r.client.Users.UpdateUser(juju.UpdateUserInput{
		Name:     data.Name.ValueString(),
		Password: data.Password.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update user, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("updated user resource %q", data.Name))

	// Save updated data into Terraform state, save a new copy for
	// update functionality.
	// The juju api as written here cannot update the Display Name,
	// take care with the optional value.
	plan := userResourceModel{
		Name:        types.StringValue(data.Name.ValueString()),
		DisplayName: data.DisplayName,
		Password:    types.StringValue(data.Password.ValueString()),
		ID:          types.StringValue(newIDFromUserName(data.Name.ValueString())),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
func (r *userResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data userResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.Users.DestroyUser(juju.DestroyUserInput{
		Name: userNameFromID(data.ID.ValueString()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete user, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("deleted user resource %q", data.Name.ValueString()))
}

func (r *userResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ID is 'user:<username'
func newIDFromUserName(value string) string {
	return fmt.Sprintf("user:%s", value)
}

func userNameFromID(value string) string {
	return strings.Split(value, ":")[1]
}
