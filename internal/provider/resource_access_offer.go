// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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

	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &accessOfferResource{}
var _ resource.ResourceWithConfigure = &accessOfferResource{}
var _ resource.ResourceWithImportState = &accessOfferResource{}
var _ resource.ResourceWithConfigValidators = &accessOfferResource{}

func NewAccessOfferResource() resource.Resource {
	return &accessOfferResource{}
}

type accessOfferResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type accessOfferResourceOffer struct {
	OfferURL types.String `tfsdk:"offer_url"`
	Users    types.Set    `tfsdk:"users"`
	Access   types.String `tfsdk:"access"`

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
		Description: "A resource that represent a Juju Access Offer.",
		Attributes: map[string]schema.Attribute{
			"access": schema.StringAttribute{
				Description: "Level of access to grant. Changing this value will replace the Terraform resource. Valid access levels are described at https://juju.is/docs/juju/manage-offers#control-access-to-an-offer",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("admin", "read", "consume"),
				},
			},
			"users": schema.SetAttribute{
				Description: "List of users to grant access.",
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

	// Get the users
	var users []string
	resp.Diagnostics.Append(plan.Users.ElementsAs(ctx, &users, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the offer
	offerURLStr := plan.OfferURL.ValueString()
	response, err := a.client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: offerURLStr,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access offer resource, got error: %s", err))
		return
	}
	a.trace(fmt.Sprintf("read offer %q at %q", response.Name, response.OfferURL))

	accessStr := plan.Access.ValueString()
	// Call Offers.GrantOffer
	for _, user := range users {
		err := a.client.Offers.GrantOffer(juju.GrantOfferInput{
			User:     user,
			Access:   accessStr,
			OfferURL: offerURLStr,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create access offer resource, got error: %s", err))
			return
		}
	}
	plan.ID = types.StringValue(newAccessOfferIDFrom(offerURLStr, accessStr, users))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessOfferResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
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

	// Retrieve information from the plan
	offerURL, access, stateUsers := retrieveAccessOfferDataFromID(ctx, plan.ID, plan.Users, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get user/access info from Offer
	response, err := a.client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: offerURL,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read offer, got error: %s", err))
		return
	}
	a.trace(fmt.Sprintf("read juju offer %q", offerURL))

	// Add to state the ones in the state and set in the offer
	plan.OfferURL = types.StringValue(offerURL)
	plan.Access = types.StringValue(access)
	var users []string
	for _, user := range stateUsers {
		for _, offerUserDetail := range response.Users {
			if user == offerUserDetail.UserName && string(offerUserDetail.Access) == access {
				users = append(users, offerUserDetail.UserName)
			}
		}
	}
	uss, errDiag := basetypes.NewSetValueFrom(ctx, types.StringType, users)
	plan.Users = uss
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a *accessOfferResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Update
}

func (a *accessOfferResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Delete
}

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
func (r *accessOfferResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	// JAAS users should use juju_jaas_access_offer instead.
	return []resource.ConfigValidator{
		NewAvoidJAASValidator(r.client, "juju_jaas_access_offer"),
	}
}

func (a *accessOfferResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	IDstr := req.ID
	if len(strings.Split(IDstr, ":")) != 3 {
		resp.Diagnostics.AddError(
			"ImportState Failure",
			fmt.Sprintf("Malformed AccessOffer ID %q, "+
				"please use format '<offer URL>:<access>:<user1,user1>'", IDstr),
		)
		return
	}
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (a *accessOfferResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if a.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(a.subCtx, LogResourceAccessOffer, msg, additionalFields...)
}

func newAccessOfferIDFrom(offerURLStr string, accessStr string, users []string) string {
	return fmt.Sprintf("%s:%s:%s", offerURLStr, accessStr, strings.Join(users, ","))
}

func retrieveAccessOfferDataFromID(ctx context.Context, ID types.String, users types.Set, diag *diag.Diagnostics) (string, string,
	[]string) {
	resID := strings.Split(ID.ValueString(), ":")
	if len(resID) < 2 {
		diag.AddError("Malformed ID", fmt.Sprintf("AccessOffer ID %q is malformed, "+
			"please use the format '<offerURL>:<access>:<user1,user2>'", resID))
		return "", "", nil
	}
	stateUsers := []string{}
	if len(resID) == 3 {
		stateUsers = strings.Split(resID[2], ",")
	} else {
		// Note: Is this still valid?
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
