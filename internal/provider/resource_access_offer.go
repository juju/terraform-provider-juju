// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/juju/core/permission"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &accessOfferResource{}
var _ resource.ResourceWithConfigure = &accessOfferResource{}
var _ resource.ResourceWithImportState = &accessOfferResource{}
var _ resource.ResourceWithConfigValidators = &accessOfferResource{}
var _ resource.ResourceWithValidateConfig = &accessOfferResource{}

// NewAccessOfferResource returns a new instance of the Access Offer resource.
func NewAccessOfferResource() resource.Resource {
	return &accessOfferResource{}
}

type accessOfferResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type accessOfferResourceOffer struct {
	OfferURL     types.String `tfsdk:"offer_url"`
	AdminUsers   types.Set    `tfsdk:"admin"`
	ConsumeUsers types.Set    `tfsdk:"consume"`
	ReadUsers    types.Set    `tfsdk:"read"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the access offer resource.
func (a *accessOfferResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_offer"
}

// Schema defines the schema for the access offer resource.
func (a *accessOfferResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Access Offer. Warning: Do not repeat users across different access levels.",
		Attributes: map[string]schema.Attribute{
			string(permission.AdminAccess): schema.SetAttribute{
				Description: "List of users to grant admin access. \"admin\" user is not allowed.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "user must be a valid Juju username")),
				},
			},
			string(permission.ConsumeAccess): schema.SetAttribute{
				Description: "List of users to grant consume access. \"admin\" user is not allowed.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "user must be a valid Juju username")),
				},
			},
			string(permission.ReadAccess): schema.SetAttribute{
				Description: "List of users to grant read access. \"admin\" user is not allowed.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "user must be a valid Juju username")),
				},
			},
			// ID required for imports
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"offer_url": schema.StringAttribute{
				Description: "The url of the offer for access management. If this is changed the resource will be deleted and a new resource will be created.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(func(s string) bool {
						_, err := crossmodel.ParseOfferURL(s)
						return err == nil
					}, "offer_url must be a valid offer string."),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

// Create attempts to grant access to the offer.
func (a *accessOfferResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access offer", "create")
		return
	}
	var plan accessOfferResourceOffer

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users to grant admin
	var adminUsers []string
	if !plan.AdminUsers.IsNull() {
		resp.Diagnostics.Append(plan.AdminUsers.ElementsAs(ctx, &adminUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant consume
	var consumeUsers []string
	if !plan.ConsumeUsers.IsNull() {
		resp.Diagnostics.Append(plan.ConsumeUsers.ElementsAs(ctx, &consumeUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant read
	var readUsers []string
	if !plan.ReadUsers.IsNull() {
		resp.Diagnostics.Append(plan.ReadUsers.ElementsAs(ctx, &readUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Call Offers.GrantOffer
	users := make(map[permission.Access][]string)
	users[permission.ConsumeAccess] = consumeUsers
	users[permission.ReadAccess] = readUsers
	users[permission.AdminAccess] = adminUsers

	for access, users := range users {
		err := a.client.Offers.GrantOffer(&juju.GrantRevokeOfferInput{
			Users:    users,
			Access:   string(access),
			OfferURL: plan.OfferURL.ValueString(),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access offer resource, got error: %s", err))
			return
		}
	}

	// Set ID as the offer URL
	plan.ID = plan.OfferURL

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read reads users and permissions granted to the offer
func (a *accessOfferResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access offer", "read")
		return
	}
	var state accessOfferResourceOffer

	// Get the Terraform state
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get information from ID
	offerURL := state.ID.ValueString()

	// Get user/access info from Offer
	response, err := a.client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: offerURL,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read offer %s, got error: %s", offerURL, err))
		return
	}
	a.trace(fmt.Sprintf("read juju offer %q", offerURL))

	// Create the map
	users := make(map[permission.Access][]string)
	users[permission.ConsumeAccess] = []string{}
	users[permission.ReadAccess] = []string{}
	users[permission.AdminAccess] = []string{}
	for _, offerUserDetail := range response.Users {
		if offerUserDetail.UserName == "everyone@external" || offerUserDetail.UserName == "admin" {
			continue
		}

		if _, ok := users[offerUserDetail.Access]; !ok {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("User %s has unexpected access %s", offerUserDetail.UserName, offerUserDetail.Access))
			return
		}

		users[offerUserDetail.Access] = append(users[offerUserDetail.Access], offerUserDetail.UserName)
	}

	// Save admin users to state
	adminUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.AdminAccess])
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.AdminUsers = adminUsersSet
	// Save consume users to state
	consumeUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.ConsumeAccess])
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ConsumeUsers = consumeUsersSet
	// Save read users to state
	readUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.ReadAccess])
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.ReadUsers = readUsersSet
	// Set the plan onto the Terraform state
	state.OfferURL = types.StringValue(offerURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update attempts to update the access to the offer.
func (a *accessOfferResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// todo
}

// Delete remove access to the offer according to the resource.
func (a *accessOfferResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// todo
}

// Configure sets the access offer resource with provider data.
func (a *accessOfferResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	a.subCtx = tflog.NewSubsystem(ctx, LogResourceAccessOffer)
}

// ConfigValidators sets validators for the resource.
func (a *accessOfferResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	// JAAS users should use juju_jaas_access_offer instead.
	return []resource.ConfigValidator{
		NewAvoidJAASValidator(a.client, "juju_jaas_access_offer"),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot(string(permission.AdminAccess)),
			path.MatchRoot(string(permission.ConsumeAccess)),
			path.MatchRoot(string(permission.ReadAccess)),
		),
	}
}

func (a *accessOfferResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	// TODO this validation does not work in case user name depends on the output of other resource
	var configData accessOfferResourceOffer

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users to grant admin
	var adminUsers []string
	if !configData.AdminUsers.IsNull() {
		resp.Diagnostics.Append(configData.AdminUsers.ElementsAs(ctx, &adminUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant consume
	var consumeUsers []string
	if !configData.ConsumeUsers.IsNull() {
		resp.Diagnostics.Append(configData.ConsumeUsers.ElementsAs(ctx, &consumeUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant read
	var readUsers []string
	if !configData.ReadUsers.IsNull() {
		resp.Diagnostics.Append(configData.ReadUsers.ElementsAs(ctx, &readUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	combinedUsers := append(append(adminUsers, consumeUsers...), readUsers...)
	// Validate if there are repeated user
	if slices.Contains(combinedUsers, "admin") {
		resp.Diagnostics.AddAttributeError(path.Root("offer_url"), "Attribute Error", "\"admin\" user is not allowed")
	}
	// Validate if there are repeated user
	slices.Sort(combinedUsers)
	originalCount := len(combinedUsers)
	compactedUsers := slices.Compact(combinedUsers)
	compactedCount := len(compactedUsers)
	if originalCount != compactedCount {
		resp.Diagnostics.AddAttributeError(path.Root("offer_url"), "Attribute Error", "do not repeat users across different access levels")
	}

}

// ImportState import existing resource to the state.
func (a *accessOfferResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (a *accessOfferResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if a.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(a.subCtx, LogResourceAccessOffer, msg, additionalFields...)
}
