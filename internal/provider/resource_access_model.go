// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

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

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
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
func (a *accessModelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	// Create the local logging subsystem here, using the TF context when creating it.
	a.subCtx = tflog.NewSubsystem(ctx, LogResourceAccessModel)
}

func (a *accessModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access model", "create")
		return
	}
	var plan accessModelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users
	var users []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &users, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelNameStr := plan.Model.ValueString()
	accessStr := plan.Access.ValueString()
	// Call Models.GrantModel
	for _, user := range users {
		err := a.client.Models.GrantModel(juju.GrantModelInput{
			User:      user,
			Access:    accessStr,
			ModelName: modelNameStr,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access model resource, got error: %s", err))
			return
		}
	}
	plan.ID = types.StringValue(newAccessModelIDFrom(modelNameStr, accessStr, users))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access model", "read")
		return
	}
	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, access, stateUsers := retrieveAccessModelDataFromID(ctx, plan.ID, plan.Users, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := a.client.Users.ModelUserInfo(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read access model resource, got error: %s", err))
		return
	}

	plan.Model = types.StringValue(modelName)
	plan.Access = types.StringValue(access)

	var users []string

	for _, user := range stateUsers {
		for _, modelUser := range response.ModelUserInfo {
			if user == modelUser.UserName && string(modelUser.Access) == access {
				users = append(users, modelUser.UserName)
			}
		}
	}

	uss, errDiag := basetypes.NewListValueFrom(ctx, types.StringType, users)
	plan.Users = uss
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}

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
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access model", "update")
		return
	}

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
	access := state.Access.ValueString()
	var missingUserList []string
	var addedUserList []string

	// Get the users that are in the planned state
	var planUsers []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &planUsers, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Check if the users has changed
	if !plan.Users.Equal(state.Users) {
		anyChange = true

		// Get the users that are in the current state
		var stateUsers []string
		resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &stateUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		missingUserList = getMissingUsers(stateUsers, planUsers)
		addedUserList = getAddedUsers(stateUsers, planUsers)
	}

	// Check if access has changed
	if !plan.Access.Equal(state.Access) {
		anyChange = true
		access = plan.Access.ValueString()
	}

	if !anyChange {
		a.trace("Update is returning without any changes.")
		return
	}

	modelName, oldAccess, _ := retrieveAccessModelDataFromID(ctx, state.ID, state.Users, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	err := a.client.Models.UpdateAccessModel(juju.UpdateAccessModelInput{
		ModelName: modelName,
		OldAccess: oldAccess,
		Grant:     addedUserList,
		Revoke:    missingUserList,
		Access:    access,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access model resource, got error: %s", err))
	}
	a.trace(fmt.Sprintf("updated access model resource for model %q", modelName))

	plan.ID = types.StringValue(newAccessModelIDFrom(modelName, access, planUsers))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access model", "delete")
		return
	}

	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users
	var stateUsers []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &stateUsers, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := a.client.Models.DestroyAccessModel(juju.DestroyAccessModelInput{
		ModelName: plan.Model.ValueString(),
		Revoke:    stateUsers,
		Access:    plan.Access.ValueString(),
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
	IDstr := req.ID
	if len(strings.Split(IDstr, ":")) != 3 {
		resp.Diagnostics.AddError(
			"ImportState Failure",
			fmt.Sprintf("Malformed AccessModel ID %q, "+
				"please use format '<modelname>:<access>:<user1,user1>'", IDstr),
		)
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (a *accessModelResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if a.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(a.subCtx, LogResourceAccessModel, msg, additionalFields...)
}

func newAccessModelIDFrom(modelNameStr string, accessStr string, users []string) string {
	return fmt.Sprintf("%s:%s:%s", modelNameStr, accessStr, strings.Join(users, ","))
}

func retrieveAccessModelDataFromID(ctx context.Context, ID types.String, users types.List, diag *diag.Diagnostics) (string, string,
	[]string) {
	resID := strings.Split(ID.ValueString(), ":")
	if len(resID) < 2 {
		diag.AddError("Malformed ID", fmt.Sprintf("AccessModel ID %q is malformed, "+
			"please use the format '<modelname>:<access>:<user1,user1>'", resID))
		return "", "", nil
	}
	stateUsers := []string{}
	if len(resID) == 3 {
		stateUsers = strings.Split(resID[2], ",")
	} else {
		// In 0.8.0 sdk2 version of the provider, the implementation of the access model
		// resource had a bug where it didn't contain the users. So we accommodate upgrades
		// from that by attempting to get the users from the state if the ID doesn't contain
		// any users (which happens only when coming from the previous version because the
		// ID is a computed field).
		diag.Append(users.ElementsAs(ctx, &stateUsers, false)...)
		if diag.HasError() {
			return "", "", nil
		}
	}

	return resID[0], resID[1], stateUsers
}
