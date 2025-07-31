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
		if offerUserDetail.UserName == "everyone@external" || offerUserDetail.UserName == "admin" {
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
	err = processRevokeReadUsers(plan.OfferURL.ValueString(), readStateUsers, readPlanUsers, consumeStateUsers, adminStateUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}
	err = processRevokeConsumeUser(plan.OfferURL.ValueString(), consumeStateUsers, readPlanUsers, consumePlanUsers, adminPlanUsers, a.client)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update access offer resource, got error: %s", err))
		return
	}
	err = processRevokeAdminUser(plan.OfferURL.ValueString(), adminStateUsers, readPlanUsers, consumePlanUsers, adminPlanUsers, a.client)
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

// processRevokeReadUsers processes the differences between the state and plan users for ReadAccess.
// If a user is in the state and in the plan, no action is needed.
// If a user is in the state, and in the plan for consume access, no action is needed.
// If a user is in the state, and in the plan for admin access, no action is needed.
// If a user is in the state, and not planned for consume or admin access, it revokes read access (which is equivalent to remove all accesses).
func processRevokeReadUsers(offerURL string, readStateUsers, readPlanUsers, consumePlanUsers, adminPlanUsers []string, client *juju.Client) error {
	for _, readUser := range readStateUsers {
		switch {
		case slices.Contains(readPlanUsers, readUser):
		case slices.Contains(consumePlanUsers, readUser):
		case slices.Contains(adminPlanUsers, readUser):
		default:
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{readUser},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return fmt.Errorf("unable to revoke read access for user %s: %w", readUser, err)
			}
		}
	}
	return nil
}

// processRevokeConsumeUser processes the differences between the state and plan users for ConsumeAccess.
// If a user is in the state and in the plan, no action is needed.
// If a user is in the state, and in the plan for read access, it revokes consume access.
// If a user is in the state, and in the plan for admin access, it does nothing.
// If a user is in the state, and not planned for read or admin access, it revokes read access (which is equivalent to remove all accesses).
func processRevokeConsumeUser(offerURL string, consumeStateUsers, readPlanUsers, consumePlanUsers, adminPlanUsers []string, client *juju.Client) error {
	for _, consumeUser := range consumeStateUsers {
		switch {
		// if the user is in the plan for admin access, do nothing.
		case slices.Contains(adminPlanUsers, consumeUser):
		case slices.Contains(consumePlanUsers, consumeUser):
		// if the user is in the plan for read access, revoke consume access.
		case slices.Contains(readPlanUsers, consumeUser):
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{consumeUser},
				Access:   string(permission.ConsumeAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return err
			}
		// if the user is not in the plan for read or admin access, revoke read access.
		default:
			// If the user is not in the plan for read or admin access, revoke read access.
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{consumeUser},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return fmt.Errorf("unable to revoke read access for user %s: %w", consumeUser, err)
			}
		}
	}
	return nil
}

// processRevokeAdminUser processes the differences between the state and plan users for AdminAccess.
// If a user is in the state and in the plan, no action is needed.
// If a user is in the state, and in the plan for consume access, it revokes admin access.
// If a user is in the state, and in the plan for read access, it revokes consume access.
// If a user is in the state, and not planned for consume or read access, it revokes read access (which is equivalent to remove all accesses).
func processRevokeAdminUser(offerURL string, adminStateUser, readPlanUsers, consumePlanUsers, adminPlanUsers []string, client *juju.Client) error {
	for _, adminUser := range adminStateUser {
		switch {
		case slices.Contains(adminPlanUsers, adminUser):
		case slices.Contains(consumePlanUsers, adminUser):
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{adminUser},
				Access:   string(permission.AdminAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return err
			}
		case slices.Contains(readPlanUsers, adminUser):
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{adminUser},
				Access:   string(permission.ConsumeAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return err
			}

		default:
			err := client.Offers.RevokeOffer(&juju.GrantRevokeOfferInput{
				Users:    []string{adminUser},
				Access:   string(permission.ReadAccess),
				OfferURL: offerURL,
			})
			if err != nil {
				return fmt.Errorf("unable to revoke read access for user %s: %w", adminUser, err)
			}
		}
	}
	return nil
}
