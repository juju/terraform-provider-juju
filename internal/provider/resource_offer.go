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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

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

type offerResourceModelV0 struct {
	ModelName       types.String `tfsdk:"model"`
	OfferName       types.String `tfsdk:"name"`
	ApplicationName types.String `tfsdk:"application_name"`
	EndpointName    types.String `tfsdk:"endpoint"`
	URL             types.String `tfsdk:"url"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type offerResourceModelV1 struct {
	ModelName       types.String `tfsdk:"model"`
	OfferName       types.String `tfsdk:"name"`
	ApplicationName types.String `tfsdk:"application_name"`
	Endpoints       types.Set    `tfsdk:"endpoints"`
	URL             types.String `tfsdk:"url"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (o *offerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offer"
}

func (o *offerResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Offer.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model to operate in. Changing this value will cause the" +
					" offer to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
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
		Version: 1,
	}
}

func (o *offerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if o.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "offer", "create")
		return
	}

	var plan offerResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName := plan.ModelName.ValueString()
	modelInfo, err := o.client.Models.GetModelByName(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model %q, got error: %s", modelName, err))
		return
	}
	// TODO (cderici): Leaking Juju info here:
	// 1 - GetModelByName above returns *params.ModelInfo
	// 2 - we don't return tag trimmed so provider has to know juju.PrefixUser etc. Make a Tag type and have a Tag.
	// Id() method.
	modelOwner := strings.TrimPrefix(modelInfo.OwnerTag, juju.PrefixUser)

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

	response, errs := o.client.Offers.CreateOffer(&juju.CreateOfferInput{
		ModelName:       modelName,
		ModelOwner:      modelOwner,
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
	var state offerResourceModelV1

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	response, err := o.client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.Append(handleOfferNotFoundError(ctx, err, &resp.State)...)
		return
	}

	o.trace(fmt.Sprintf("read offer %q at %q", response.Name, response.OfferURL))

	state.ModelName = types.StringValue(response.ModelName)
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
	var plan offerResourceModelV1

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := o.client.Offers.DestroyOffer(&juju.DestroyOfferInput{
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

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	o.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	o.subCtx = tflog.NewSubsystem(ctx, LogResourceOffer)
}

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

// UpgradeState upgrades the state of the offer resource.
// This is used to handle changes in the resource schema between versions.
func (o *offerResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// Upgrade from `endpoint` to `endpoints` attribute.
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						Required: true,
					},
					"name": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"application_name": schema.StringAttribute{
						Required: true,
					},
					"endpoint": schema.StringAttribute{
						Optional: true,
					},
					"url": schema.StringAttribute{
						Computed: true,
					},
					"id": schema.StringAttribute{
						Computed: true,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData offerResourceModelV0

				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)

				if resp.Diagnostics.HasError() {
					return
				}

				endpoints := []string{}
				if !priorStateData.EndpointName.IsNull() {
					endpoints = append(endpoints, priorStateData.EndpointName.ValueString())
				}

				endpointsSet, diags := types.SetValueFrom(ctx, types.StringType, endpoints)
				if diags.HasError() {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert endpoints to set, got error: %s", diags))
					return
				}
				upgradedStateData := offerResourceModelV1{
					ModelName:       priorStateData.ModelName,
					OfferName:       priorStateData.OfferName,
					ApplicationName: priorStateData.ApplicationName,
					Endpoints:       endpointsSet,
					URL:             priorStateData.URL,
					ID:              priorStateData.ID,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}
