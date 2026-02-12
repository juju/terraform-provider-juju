// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &offerResource{}
var _ resource.ResourceWithConfigure = &offerResource{}
var _ resource.ResourceWithImportState = &offerResource{}

func NewOfferResource() resource.Resource {
	return &offerResource{}
}

type offerResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type offerResourceModel struct {
	OfferName       types.String `tfsdk:"name"`
	ApplicationName types.String `tfsdk:"application_name"`
	URL             types.String `tfsdk:"url"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type offerResourceModelV2 struct {
	offerResourceModel
	ModelUUID types.String `tfsdk:"model_uuid"`
	Endpoints types.Set    `tfsdk:"endpoints"`
}

func (o *offerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offer"
}

func (o *offerResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version:     2,
		Description: "A resource that represent a Juju Offer.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model to operate in. Changing this value will cause the" +
					" offer to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the offer. Changing this value will cause the offer" +
					" to be destroyed and recreated by terraform.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"application_name": schema.StringAttribute{
				Description: "The name of the application. Changing this value will cause the offer" +
					" to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoints": schema.SetAttribute{
				ElementType: types.StringType,
				Description: "The endpoint names. Changing this value will cause the offer" +
					" to be destroyed and recreated by terraform.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.RequiresReplace(),
				},
				Required: true,
			},
			"url": schema.StringAttribute{
				Description: "The offer URL.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
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

func (o *offerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if o.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "offer", "create")
		return
	}

	var plan offerResourceModelV2

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID := plan.ModelUUID.ValueString()

	//here we verify if the name property is set, if not, set it to the application name
	offerName := plan.OfferName.ValueString()
	if offerName == "" {
		offerName = plan.ApplicationName.ValueString()
	}

	var endpoints []string
	diag := plan.Endpoints.ElementsAs(ctx, &endpoints, false)
	if diag.HasError() {
		resp.Diagnostics.Append(diag...)
		return
	}

	response, errs := o.client.Offers.CreateOffer(ctx, &juju.CreateOfferInput{
		ModelUUID:       modelUUID,
		Name:            offerName,
		ApplicationName: plan.ApplicationName.ValueString(),
		Endpoints:       endpoints,
		OfferOwner:      o.client.Username(),
	})
	if errs != nil {
		// TODO 10-Aug-2023
		// Fix client.Offers.CreateOffer to only return a single error. The juju api method
		// accepts multiple input, thus returns multiple errors, one per input. The internal/juju
		// code should handle this without leaking to the provider code.
		//
		// Why do we pass the CreateOfferInput as a pointer?
		for _, err := range errs {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create offer, got error: %s", err))
		}
		return
	}
	o.trace(fmt.Sprintf("create offer %q at %q", response.Name, response.OfferURL))

	plan.OfferName = types.StringValue(response.Name)
	plan.URL = types.StringValue(response.OfferURL)
	plan.ID = types.StringValue(response.OfferURL)

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (o *offerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if o.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "offer", "read")
		return
	}
	var state offerResourceModelV2

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := o.client.Offers.ReadOffer(ctx, &juju.ReadOfferInput{
		OfferURL:     state.ID.ValueString(),
		GetModelUUID: true,
	})
	if err != nil {
		resp.Diagnostics.Append(handleOfferNotFoundError(ctx, err, &resp.State)...)
		return
	}

	o.trace(fmt.Sprintf("read offer %q at %q", response.Name, response.OfferURL))

	state.ModelUUID = types.StringValue(response.ModelUUID)
	state.OfferName = types.StringValue(response.Name)
	state.ApplicationName = types.StringValue(response.ApplicationName)
	endpointSet, diags := types.SetValueFrom(ctx, types.StringType, response.Endpoints)
	if diags.HasError() {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert endpoints to set, got error: %s", err))
		return
	}
	state.Endpoints = endpointSet
	state.URL = types.StringValue(response.OfferURL)
	state.ID = types.StringValue(response.OfferURL)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (o *offerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Everything is always replaced, so Update should not be called.
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
//
// Juju refers to deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func (o *offerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if o.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "offer", "delete")
		return
	}
	var plan offerResourceModelV2

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := o.client.Offers.DestroyOffer(ctx, &juju.DestroyOfferInput{
		OfferURL: plan.URL.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete offer, got error: %s", err))
		return
	}
	o.trace(fmt.Sprintf("delete offer resource %q", plan.URL))
}

func (o *offerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	o.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	o.subCtx = tflog.NewSubsystem(ctx, LogResourceOffer)
}

// ImportState imports the resource state from the given ID.
// The ID is expected to be `offer_url`.
func (o *offerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (o *offerResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if o.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(o.subCtx, LogResourceOffer, msg, additionalFields...)
}

func isOfferNotFound(err error) bool {
	return strings.Contains(err.Error(), "expected to find one result for url")
}

func handleOfferNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if isOfferNotFound(err) {
		// Offer manually removed
		st.RemoveResource(ctx)
		return diag.Diagnostics{}
	}

	var diags diag.Diagnostics
	diags.AddError("Client Error", err.Error())
	return diags
}
