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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &accessModelResource{}
var _ resource.ResourceWithConfigure = &accessModelResource{}
var _ resource.ResourceWithImportState = &accessModelResource{}

func NewAccessModelResource() resource.Resource {
	return &accessModelResource{}
}

type accessModelResource struct {
	client *juju.Client
}

type accessModelResourceModel struct {
	Model  types.String `tfsdk:"model"`
	Users  types.List   `tfsdk:"users"`
	Access types.String `tfsdk:"access"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (a *accessModelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_model"
}

func (a *accessModelResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Access Model.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model for access management",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"users": schema.ListAttribute{
				Description: "List of users to grant access to",
				Required:    true,
				ElementType: types.StringType,
			},
			"access": schema.StringAttribute{
				Description: "Type of access to the model",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("admin", "read", "write"),
				},
			},
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (a *accessModelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	a.client = client
}

func (a *accessModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessModelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users
	var users []string
	plan.Users.ElementsAs(ctx, users, false)

	modelNameStr := plan.Model.ValueString()
	// Get the modelUUID to call Models.GrantModel
	uuid, err := a.client.Models.ResolveModelUUID(modelNameStr)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access model resource, got error: %s", err))
		return
	}
	modelUUIDs := []string{uuid}

	accessStr := plan.Access.ValueString()
	// Call Models.GrantModel
	for _, user := range users {
		err := a.client.Models.GrantModel(juju.GrantModelInput{
			User:       user,
			Access:     accessStr,
			ModelUUIDs: modelUUIDs,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access model resource, got error: %s", err))
			return
		}
	}
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s:%s", modelNameStr, accessStr, strings.Join(users, ",")))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resID := strings.Split(plan.ID.ValueString(), ":")

	// Get the users
	stateUsers := strings.Split(resID[2], ",")

	// Prevent a segfault if client is not yet configured
	if a.client == nil {
		resp.Diagnostics.AddError(
			"Provider Error, Client Not Configured",
			"Unable to read access model resource. Expected configured Juju Client. "+
				"Please report this issue to the provider developers.",
		)
		return
	}
	uuid, err := a.client.Models.ResolveModelUUID(resID[0])
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read access model resource, got error: %s", err))
		return
	}

	response, err := a.client.Users.ModelUserInfo(uuid)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read access model resource, got error: %s", err))
		return
	}

	plan.Model = types.StringValue(resID[0])
	plan.Access = types.StringValue(resID[1])

	var users []string

	for _, user := range stateUsers {
		for _, modelUser := range response.ModelUserInfo {
			if user == modelUser.UserName && string(modelUser.Access) == resID[1] {
				users = append(users, modelUser.UserName)
			}
		}
	}

	uss, errDiag := basetypes.NewListValueFrom(ctx, types.StringType, users)
	plan.Users = uss
	resp.Diagnostics.Append(errDiag...)
	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Update on the access model supports three cases
// access and users both changed:
// for missing users - revoke access
// for changed users - apply new access
// users changed:
// for missing users - revoke access
// for new users - apply access
// access changed - apply new access
func (a *accessModelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	anyChange := false

	// items that could be changed
	var newAccess string
	var missingUserList []string
	var addedUserList []string

	// Check if the users has changed
	if !plan.Users.Equal(state.Users) {
		anyChange = true

		// Get the users that are in the current state
		var stateUsers []string
		state.Users.ElementsAs(ctx, stateUsers, false)

		// Get the users that are in the planned states
		var planUsers []string
		plan.Users.ElementsAs(ctx, planUsers, false)

		missingUserList = getMissingUsers(stateUsers, planUsers)
		addedUserList = getAddedUsers(stateUsers, planUsers)
	}

	// Check if access has changed
	if !plan.Access.Equal(state.Access) {
		anyChange = true
		newAccess = plan.Access.ValueString()
	}

	if !anyChange {
		return
	}

	// Prevent a segfault if client is not yet configured
	if a.client == nil {
		resp.Diagnostics.AddError(
			"Provider Error, Client Not Configured",
			"Unable to update access model resource. Expected configured Juju Client. "+
				"Please report this issue to the provider developers.",
		)
		return
	}
	err := a.client.Models.UpdateAccessModel(juju.UpdateAccessModelInput{
		Model:  plan.ID.ValueString(),
		Grant:  addedUserList,
		Revoke: missingUserList,
		Access: newAccess,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access model resource, got error: %s", err))
	}
	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users
	var stateUsers []string
	plan.Users.ElementsAs(ctx, stateUsers, false)

	// Prevent a segfault if client is not yet configured
	if a.client == nil {
		resp.Diagnostics.AddError(
			"Provider Error, Client Not Configured",
			"Unable to delete access model resource. Expected configured Juju Client. "+
				"Please report this issue to the provider developers.",
		)
		return
	}
	err := a.client.Models.DestroyAccessModel(juju.DestroyAccessModelInput{
		Model:  plan.ID.ValueString(),
		Revoke: stateUsers,
		Access: plan.Access.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete access model resource, got error: %s", err))
	}
}

func getMissingUsers(oldUsers, newUsers []string) []string {
	var missing []string
	for _, user := range oldUsers {
		found := false
		for _, newUser := range newUsers {
			if user == newUser {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, user)
		}
	}
	return missing
}

func getAddedUsers(oldUsers, newUsers []string) []string {
	var added []string
	for _, user := range newUsers {
		found := false
		for _, oldUser := range oldUsers {
			if user == oldUser {
				found = true
				break
			}
		}
		if !found {
			added = append(added, user)
		}
	}
	return added
}

func (a *accessModelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
