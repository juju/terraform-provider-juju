// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

type offerLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

func NewOfferLister() list.ListResourceWithConfigure {
	return &offerLister{}
}

func (r *offerLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.client = provider.Client
	r.config = provider.Config
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceOffer)
}

func (r *offerLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offer"
}

type offerListConfigModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	OfferURL  types.String `tfsdk:"offer_url"`
}

func (r *offerLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": listschema.StringAttribute{
				Description: "Filter by offers in a specific model.",
				Optional:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"offer_url": listschema.StringAttribute{
				Description: "Filter by a specific model URL.",
				Optional:    true,
				Validators: []validator.String{
					NewValidatorOfferURL(),
				},
			},
		},
	}
}

func (r *offerLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	stream.Results = func(push func(list.ListResult) bool) {
		result := req.NewListResult(ctx)

		// Read list configuration
		var config offerListConfigModel
		result.Diagnostics.Append(req.Config.Get(ctx, &config)...)
		if result.Diagnostics.HasError() {
			return
		}

		// Prepare input for ListOffers
		input := &juju.ListOffersInput{}
		if !config.ModelUUID.IsNull() {
			input.ModelUUID = config.ModelUUID.ValueString()
		}
		if !config.OfferURL.IsNull() {
			input.OfferURL = config.OfferURL.ValueString()
		}

		// List offers
		offers, err := r.client.Offers.ListOffers(input)
		if err != nil {
			result.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list offers, got error: %s", err))
			return
		}

		for _, offer := range offers {
			result.DisplayName = offer.OfferURL
			identity := offerResourceIdentityModel{
				ID: types.StringValue(offer.OfferURL),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				return
			}

			if req.IncludeResource {
				resource, err := r.getOfferResource(ctx, offer)
				if err.HasError() {
					result.Diagnostics.Append(err...)
					return
				}
				result.Diagnostics.Append(result.Resource.Set(ctx, resource)...)
				if result.Diagnostics.HasError() {
					return
				}
			}
			if !push(result) {
				return
			}
		}
	}
}

func (r *offerLister) getOfferResource(ctx context.Context, offer juju.ListOffersOutput) (offerResourceModelV2, diag.Diagnostics) {
	resource := offerResourceModelV2{}

	diags := diag.Diagnostics{}

	resource.OfferName = types.StringValue(offer.Name)
	resource.ApplicationName = types.StringValue(offer.ApplicationName)
	resource.ModelUUID = types.StringValue(offer.ModelUUID)
	resource.URL = types.StringValue(offer.OfferURL)
	resource.ID = types.StringValue(offer.OfferURL)

	endpointSet, errDiag := types.SetValueFrom(ctx, types.StringType, offer.Endpoints)
	diags.Append(errDiag...)
	if diags.HasError() {
		return offerResourceModelV2{}, diags
	}
	resource.Endpoints = endpointSet

	return resource, diags
}
