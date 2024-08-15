// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"regexp"

	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var (
	basicEmailValidationRe = regexp.MustCompile(".+@.+")
	avoidAtSymbolRe        = regexp.MustCompile("^[^@]*$")
)

// Getter is used to get details from a plan or state object.
// Implemented by Terraform's [State] and [Plan] types.
type Getter interface {
	Get(ctx context.Context, target interface{}) diag.Diagnostics
}

// resourceInfo defines how the [genericJAASAccessResource] can query for information
// on the target object.
type resourceInfo interface {
	Identity(ctx context.Context, getter Getter, diag *diag.Diagnostics) string
}

// genericJAASAccessResource is a generic resource that can be used for creating access rules with JAAS.
// Other types should embed this struct and implement their own metadata and schema methods. The schema
// should build on top of [PartialAccessSchema].
// The embedded struct requires a targetInfo interface to enable fetching the target object in the relation.
type genericJAASAccessResource struct {
	client          *juju.Client
	targetInfo      resourceInfo
	resourceLogName string

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// genericJAASAccessModel represents a partial generic object for access management.
// This struct should be embedded into a struct that contains a field for a target object (normally a name or UUID).
// Note that service accounts are treated as users but kept as a separate field for improved validation.
type genericJAASAccessModel struct {
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Access          types.String `tfsdk:"access"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// ConfigValidators sets validators for the resource.
func (r *genericJAASAccessResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		RequiresJAASValidator{Client: r.client},
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("users"),
			path.MatchRoot("groups"),
			path.MatchRoot("service_accounts"),
		),
	}
}

// partialAccessSchema returns a map of schema attributes for a JAAS access resource.
// Access resources should use this schema and add any additional attributes e.g. name or uuid.
func (r *genericJAASAccessResource) partialAccessSchema() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"access": schema.StringAttribute{
			Description: "Level of access to grant. Changing this value will replace the Terraform resource.",
			Required:    true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.RequiresReplace(),
			},
		},
		"users": schema.SetAttribute{
			Description: "List of users to grant access.",
			Optional:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "email must be a valid Juju username")),
				setvalidator.ValueStringsAre(stringvalidator.RegexMatches(basicEmailValidationRe, "email must contain an @ symbol")),
			},
		},
		"groups": schema.SetAttribute{
			Description: "List of groups to grant access.",
			Optional:    true,
			ElementType: types.StringType,
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(jimmnames.IsValidGroupId, "group ID must be valid")),
			},
		},
		"service_accounts": schema.SetAttribute{
			Description: "List of service accounts to grant access.",
			Optional:    true,
			ElementType: types.StringType,
			// service accounts are treated as users but defined separately
			// for different validation and logic in the provider.
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(ValidatorMatchString(names.IsValidUser, "service account must be valid Juju usernames")),
				setvalidator.ValueStringsAre(stringvalidator.RegexMatches(avoidAtSymbolRe, "service account should not contain an @ symbol")),
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (a *genericJAASAccessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	a.subCtx = tflog.NewSubsystem(ctx, a.resourceLogName)
}

// Create defines how tuples for access control will be created.
func (a *genericJAASAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

}

// Read defines how tuples for access control will be read.
func (a *genericJAASAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {

}

// Update defines how tuples for access control will be updated.
func (a *genericJAASAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

}

// Delete defines how tuples for access control will be updated.
func (a *genericJAASAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {

}