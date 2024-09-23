// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/names/v5"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasAccessOfferResource{}
var _ resource.ResourceWithConfigure = &jaasAccessOfferResource{}
var _ resource.ResourceWithImportState = &jaasAccessOfferResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessOfferResource{}

// NewJAASAccessOfferResource returns a new resource for JAAS offer access.
func NewJAASAccessOfferResource() resource.Resource {
	return &jaasAccessOfferResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  offerInfo{},
		resourceLogName: LogResourceJAASAccessOffer,
	}}
}

type offerInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j offerInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (genericJAASAccessData, names.Tag) {
	offerResource := jaasAccessOfferResourceOffer{}
	diag.Append(getter.Get(ctx, &offerResource)...)
	genericInfo := genericJAASAccessData{
		ID:              offerResource.ID,
		Users:           offerResource.Users,
		Groups:          offerResource.Groups,
		ServiceAccounts: offerResource.ServiceAccounts,
		Access:          offerResource.Access,
	}
	// When importing, the offer url will be empty
	var tag names.Tag
	if offerResource.OfferUrl.ValueString() != "" {
		tag = names.NewApplicationOfferTag(offerResource.OfferUrl.ValueString())
	}
	return genericInfo, tag
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j offerInfo) Save(ctx context.Context, setter Setter, info genericJAASAccessData, tag names.Tag) diag.Diagnostics {
	offerAccess := jaasAccessOfferResourceOffer{
		OfferUrl:        basetypes.NewStringValue(tag.Id()),
		ID:              info.ID,
		Users:           info.Users,
		Groups:          info.Groups,
		ServiceAccounts: info.ServiceAccounts,
		Access:          info.Access,
	}
	return setter.Set(ctx, offerAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j offerInfo) ImportHint() string {
	return "offer-<url>:<access-level>"
}

type jaasAccessOfferResource struct {
	genericJAASAccessResource
}

type jaasAccessOfferResourceOffer struct {
	OfferUrl        types.String `tfsdk:"offer_url"`
	Users           types.Set    `tfsdk:"users"`
	ServiceAccounts types.Set    `tfsdk:"service_accounts"`
	Groups          types.Set    `tfsdk:"groups"`
	Access          types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS offer access resource.
func (a *jaasAccessOfferResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_offer"
}

// Schema defines the schema for the JAAS offer access resource.
func (a *jaasAccessOfferResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := a.partialAccessSchema()
	attributes["offer_url"] = schema.StringAttribute{
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
	}
	schema := schema.Schema{
		Description: "A resource that represent access to an offer when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
