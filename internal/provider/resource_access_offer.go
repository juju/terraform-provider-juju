// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

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

	// validate if there are overlaps or admin user
	// validation is done here considering dynamic (juju_user resource) and static values for users
	err := validateNoOverlapsNoAdmin(adminUsers, consumeUsers, readUsers)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access offer resource, got error: %s", err))
		return
	}

	// Call Offers.GrantOffer
	totalUsers := make(map[permission.Access][]string)
	totalUsers[permission.ConsumeAccess] = consumeUsers
	totalUsers[permission.ReadAccess] = readUsers
	totalUsers[permission.AdminAccess] = adminUsers

	for access, users := range totalUsers {
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
		if offerUserDetail.UserName == "everyone@external" || offerUserDetail.UserName == a.client.Username() {
			continue
		}

		if _, ok := users[offerUserDetail.Access]; !ok {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("User %s has unexpected access %s", offerUserDetail.UserName, offerUserDetail.Access))
			return
		}

		users[offerUserDetail.Access] = append(users[offerUserDetail.Access], offerUserDetail.UserName)
	}
	a.trace(fmt.Sprintf("read juju offer response %q", response))
	// Save admin users to state
	if len(users[permission.AdminAccess]) > 0 {
		adminUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.AdminAccess])
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.AdminUsers = adminUsersSet
	}
	// Save consume users to state
	if len(users[permission.ConsumeAccess]) > 0 {
		consumeUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.ConsumeAccess])
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ConsumeUsers = consumeUsersSet
	}
	// Save read users to state
	if len(users[permission.ReadAccess]) > 0 {
		readUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users[permission.ReadAccess])
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.ReadUsers = readUsersSet
	}
	// Set the plan onto the Terraform state
	state.OfferURL = types.StringValue(offerURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update attempts to update the access to the offer.
func (a *accessOfferResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access offer", "update")
		return
	}
	var plan, state accessOfferResourceOffer

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users to grant admin
	var adminPlanUsers []string
	if !plan.AdminUsers.IsNull() {
		resp.Diagnostics.Append(plan.AdminUsers.ElementsAs(ctx, &adminPlanUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant consume
	var consumePlanUsers []string
	if !plan.ConsumeUsers.IsNull() {
		resp.Diagnostics.Append(plan.ConsumeUsers.ElementsAs(ctx, &consumePlanUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Get the users to grant read
	var readPlanUsers []string
	if !plan.ReadUsers.IsNull() {
		resp.Diagnostics.Append(plan.ReadUsers.ElementsAs(ctx, &readPlanUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// validate if there are overlaps or admin user
	// validation is done here considering dynamic (juju_user resource) and static values for users
	err := validateNoOverlapsNoAdmin(adminPlanUsers, consumePlanUsers, readPlanUsers)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}

	// Get users from state
	var adminStateUsers []string
	if !state.AdminUsers.IsNull() {
		resp.Diagnostics.Append(state.AdminUsers.ElementsAs(ctx, &adminStateUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var consumeStateUsers []string
	if !state.ConsumeUsers.IsNull() {
		resp.Diagnostics.Append(state.ConsumeUsers.ElementsAs(ctx, &consumeStateUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var readStateUsers []string
	if !state.ReadUsers.IsNull() {
		resp.Diagnostics.Append(state.ReadUsers.ElementsAs(ctx, &readStateUsers, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// The process for revoking access in Juju is somewhat unintuitive.
	// If a user has admin access and should be demoted to read access,
	// we revoke their consume access, dropping them to read access.
	// Once revokes are done, granting access becomes straightforward.
	// It's important not to remove all access and then grant access
	// as this breaks access to offers.

	stateUsers, planUsers := buildUserAccessMaps(adminStateUsers, consumeStateUsers, readStateUsers, adminPlanUsers, consumePlanUsers, readPlanUsers)

	err = processAccessRevokes(plan.OfferURL.ValueString(), stateUsers, planUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}

	// grant read
	err = grantPermission(plan.OfferURL.ValueString(), string(permission.ReadAccess), readPlanUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}

	// grant consume
	err = grantPermission(plan.OfferURL.ValueString(), string(permission.ConsumeAccess), consumePlanUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}

	// grant admin
	err = grantPermission(plan.OfferURL.ValueString(), string(permission.AdminAccess), adminPlanUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}

	// Save admin users to state
	adminUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, adminPlanUsers)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.AdminUsers = adminUsersSet

	// Save consume users to state
	consumeUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, consumePlanUsers)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ConsumeUsers = consumeUsersSet

	// Save read users to state
	readUsersSet, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, readPlanUsers)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ReadUsers = readUsersSet

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete remove access to the offer according to the resource.
func (a *accessOfferResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Check first if the client is configured
	if a.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access offer", "read")
		return
	}
	var plan accessOfferResourceOffer

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
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

	totalPlanUsers := append(adminUsers, consumeUsers...)
	totalPlanUsers = append(totalPlanUsers, readUsers...)

	// Revoking against "read" guarantees that the entire access will be removed
	// instead of only decreasing the access level.
	err := a.client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
		Users:    totalPlanUsers,
		Access:   string(permission.ReadAccess),
		OfferURL: plan.OfferURL.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to destroy access offer resource, got error: %s", err))
		return
	}
}

// Configure sets the access offer resource with provider data.
func (a *accessOfferResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	resp.Diagnostics = checkControllerMode(resp.Diagnostics, provider.Config, false)
	if resp.Diagnostics.HasError() {
		return
	}
	a.client = provider.Client
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

// validateNoOverlapsNoAdmin validates that there are no overlaps between the users in different access levels
// and that the "admin" user is not present in any of the access levels.
func validateNoOverlapsNoAdmin(admin, consume, read []string) error {
	sets := map[string]struct{}{}
	for _, v := range consume {
		if v == "admin" {
			return fmt.Errorf("user admin is not allowed")
		}
		sets[v] = struct{}{}
	}
	for _, v := range read {
		if v == "admin" {
			return fmt.Errorf("user admin is not allowed")
		}
		if _, exists := sets[v]; exists {
			return fmt.Errorf("user '%s' appears in both 'consume' and 'read'", v)
		}
		sets[v] = struct{}{}
	}
	for _, v := range admin {
		if v == "admin" {
			return fmt.Errorf("user admin is not allowed")
		}
		if _, exists := sets[v]; exists {
			return fmt.Errorf("user '%s' appears in multiple roles (e.g., 'consume', 'read', 'admin')", v)
		}
	}

	return nil
}

func grantPermission(offerURL, permissionType string, planUsers []string, jujuClient *juju.Client) error {
	err := jujuClient.Offers.GrantOffer(&juju.GrantRevokeOfferInput{
		Users:    planUsers,
		Access:   permissionType,
		OfferURL: offerURL,
	})
	if err != nil {
		return err
	}
	return nil
}

// buildUserAccessMaps builds stateUsers and planUsers maps for access management.
func buildUserAccessMaps(adminStateUsers, consumeStateUsers, readStateUsers, adminPlanUsers, consumePlanUsers, readPlanUsers []string) (map[string]permission.Access, map[string]permission.Access) {
	stateUsers := map[string]permission.Access{}
	for _, u := range adminStateUsers {
		stateUsers[u] = permission.AdminAccess
	}
	for _, u := range consumeStateUsers {
		if _, ok := stateUsers[u]; !ok {
			stateUsers[u] = permission.ConsumeAccess
		}
	}
	for _, u := range readStateUsers {
		if _, ok := stateUsers[u]; !ok {
			stateUsers[u] = permission.ReadAccess
		}
	}

	planUsers := map[string]permission.Access{}
	for _, u := range adminPlanUsers {
		planUsers[u] = permission.AdminAccess
	}
	for _, u := range consumePlanUsers {
		if _, ok := planUsers[u]; !ok {
			planUsers[u] = permission.ConsumeAccess
		}
	}
	for _, u := range readPlanUsers {
		if _, ok := planUsers[u]; !ok {
			planUsers[u] = permission.ReadAccess
		}
	}
	return stateUsers, planUsers
}

// processAccessRevokes loops over all users in the state and compares their current access to the desired access.
// It calls revokeAccessIfNeeded to revoke access if needed.
func processAccessRevokes(
	offerURL string,
	stateUsers map[string]permission.Access, // user -> current access
	planUsers map[string]permission.Access, // user -> desired access (missing means remove)
	client *juju.Client,
) error {
	for user, currentAccess := range stateUsers {
		desiredAccess, exists := planUsers[user]
		if !exists {
			desiredAccess = "" // Means remove all access
		}
		if err := revokeAccessIfNeeded(offerURL, user, currentAccess, desiredAccess, client); err != nil {
			return err
		}
	}
	return nil
}

// revokeAccessIfNeeded revokes the correct permission for a user based on their current and desired access.
func revokeAccessIfNeeded(
	offerURL, user string,
	currentAccess, desiredAccess permission.Access,
	client *juju.Client,
) error {
	// No change, nothing to do
	if currentAccess == desiredAccess {
		return nil
	}

	switch currentAccess {
	case permission.AdminAccess:
		switch desiredAccess {
		case permission.ConsumeAccess:
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.AdminAccess),
				OfferURL: offerURL,
			})
		case permission.ReadAccess:
			// Only need to revoke consume access to demote from admin to read
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.ConsumeAccess),
				OfferURL: offerURL,
			})
		default: // remove all access
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
		}
	case permission.ConsumeAccess:
		switch desiredAccess {
		case permission.ReadAccess:
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.ConsumeAccess),
				OfferURL: offerURL,
			})
		case permission.AdminAccess:
			// Upgrading, nothing to revoke
			return nil
		default: // remove all access
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
		}
	case permission.ReadAccess:
		switch desiredAccess {
		case permission.ConsumeAccess, permission.AdminAccess:
			// Upgrading, nothing to revoke
			return nil
		default: // remove all access
			return client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{user},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
		}
	}
	return nil
}
