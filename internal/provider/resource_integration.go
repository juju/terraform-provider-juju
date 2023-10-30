// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &integrationResource{}
var _ resource.ResourceWithConfigure = &integrationResource{}
var _ resource.ResourceWithImportState = &integrationResource{}
var _ resource.ResourceWithValidateConfig = &integrationResource{}

func NewIntegrationResource() resource.Resource {
	return &integrationResource{}
}

type integrationResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type integrationResourceModel struct {
	ModelName   types.String `tfsdk:"model"`
	Via         types.String `tfsdk:"via"`
	Application types.Set    `tfsdk:"application"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// nestedApplication represents an element in an Application set of an
// integration resource
type nestedApplication struct {
	Name     types.String `tfsdk:"name"`
	Endpoint types.String `tfsdk:"endpoint"`
	OfferURL types.String `tfsdk:"offer_url"`
}

func (r *integrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest,
	resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *integrationResource) Configure(ctx context.Context, req resource.ConfigureRequest,
	resp *resource.ConfigureResponse) {
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
	r.client = client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceIntegration)
}

// Called during terraform validate through ValidateResourceConfig RPC
// Validates the logic in the application block in the Schema
func (r *integrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var configData integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &configData)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var apps []nestedApplication
	configData.Application.ElementsAs(ctx, &apps, false)
	for _, app := range apps {
		if app.Name.IsNull() && app.OfferURL.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("applications"), "Attribute Error", "one and only one of \"name\" or \"offer_url\" fields must be provided.")
		} else if !app.OfferURL.IsNull() && !app.Name.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("applications"), "Attribute Error", "the \"offer_url\" and \"name\" fields are mutually exclusive.")
		} else if !app.OfferURL.IsNull() && !app.Endpoint.IsNull() {
			resp.Diagnostics.AddAttributeError(path.Root("applications"), "Attribute Error", "the \"endpoint\" field can not be specified with the \"offer_url\" field.")
		}
	}
}

func (r *integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (r *integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju Integration.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model to operate in.",
				Required:    true,
			},
			"via": schema.StringAttribute{
				Description: "A comma separated list of CIDRs for outbound traffic.",
				Optional:    true,
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"application": schema.SetNestedBlock{
				Description: "The two applications to integrate.",
				Validators: []validator.Set{
					setvalidator.SizeBetween(2, 2),
					setvalidator.IsRequired(),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the application.",
							Optional:    true,
						},
						"endpoint": schema.StringAttribute{
							Description: "The endpoint name.",
							Optional:    true,
							Computed:    true,
						},
						"offer_url": schema.StringAttribute{
							Description: "The URL of a remote application.",
							Optional:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
		},
	}
}

func (r *integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "create")
		return
	}

	var plan integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelName := plan.ModelName.ValueString()

	var apps []nestedApplication
	resp.Diagnostics.Append(plan.Application.ElementsAs(ctx, &apps, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoints, offerURL, appNames, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse endpoints, got error: %s", err))
		return
	}

	var offerResponse = &juju.ConsumeRemoteOfferResponse{}
	if offerURL != nil {
		offerResponse, err = r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelName: modelName,
			OfferURL:  *offerURL,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to consume remote offer, got error: %s", err))
			return
		}
		r.trace(fmt.Sprintf("remote offer created : %q", *offerURL))
	}

	if offerResponse.SAASName != "" {
		endpoints = append(endpoints, offerResponse.SAASName)
	}

	viaCIDRs := plan.Via.ValueString()
	response, err := r.client.Integrations.CreateIntegration(&juju.IntegrationInput{
		ModelName: modelName,
		Apps:      appNames,
		Endpoints: endpoints,
		ViaCIDRs:  viaCIDRs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create integration, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("integration created on Juju between %q at %q on model %q", appNames, endpoints, modelName))

	parsedApplications := parseApplications(response.Applications)

	appsType := req.Plan.Schema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()
	parsedApps, errDiag := types.SetValueFrom(ctx, appsType, parsedApplications)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Application = parsedApps

	id := newIDForIntegrationResource(modelName, response.Applications)
	plan.ID = types.StringValue(id)

	r.trace(fmt.Sprintf("integration resource created: %q", id))
	// Write the state plan into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "read")
		return
	}

	var state integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, endpointA, endpointB, idErr := modelNameAndEndpointsFromID(state.ID.ValueString())
	if idErr.HasError() {
		resp.Diagnostics.Append(idErr...)
		return
	}

	integration := &juju.IntegrationInput{
		ModelName: modelName,
		Endpoints: []string{
			endpointA,
			endpointB,
		},
	}

	response, err := r.client.Integrations.ReadIntegration(integration)
	if err != nil {
		resp.Diagnostics.Append(handleIntegrationNotFoundError(ctx, err, &resp.State)...)
		return
	}
	r.trace(fmt.Sprintf("found integration: %v", integration))

	state.ModelName = types.StringValue(modelName)

	applications := parseApplications(response.Applications)
	appType := req.State.Schema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()
	apps, aErr := types.SetValueFrom(ctx, appType, applications)
	if aErr.HasError() {
		resp.Diagnostics.Append(aErr...)
		return
	}
	state.Application = apps

	r.trace(fmt.Sprintf("read integration resource: %v", state.ID.ValueString()))
	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "update")
		return
	}
	var plan, state integrationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName := plan.ModelName.ValueString()

	var oldEndpoints, endpoints []string
	var oldOfferURL, offerURL *string
	var err error

	if !plan.Application.Equal(state.Application) {
		var oldApps []nestedApplication
		state.Application.ElementsAs(ctx, &oldApps, false)
		oldEndpoints, oldOfferURL, _, err = parseEndpoints(oldApps)
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}

		var newApps []nestedApplication
		plan.Application.ElementsAs(ctx, &newApps, false)
		endpoints, offerURL, _, err = parseEndpoints(newApps)
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}
	}

	var offerResponse *juju.ConsumeRemoteOfferResponse
	//check if the offer url is present and is not the same as before the change
	if oldOfferURL != offerURL && !(oldOfferURL == nil && offerURL == nil) {
		if oldOfferURL != nil {
			//destroy old offer
			errs := r.client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
				ModelName: modelName,
				OfferURL:  *oldOfferURL,
			})
			if len(errs) > 0 {
				for _, v := range errs {
					resp.Diagnostics.AddError("Client Error", v.Error())
				}
				return
			}
			r.trace(fmt.Sprintf("removed offer on Juju: %q", *oldOfferURL))
		}
		if offerURL != nil {
			offerResponse, err = r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
				ModelName: modelName,
				OfferURL:  *offerURL,
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", err.Error())
				return
			}
			endpoints = append(endpoints, offerResponse.SAASName)
			r.trace(fmt.Sprintf("added offer on Juju: %q", *offerURL))
		}
	}

	viaCIDRs := plan.Via.ValueString()
	input := &juju.UpdateIntegrationInput{
		ModelName:    modelName,
		Endpoints:    endpoints,
		OldEndpoints: oldEndpoints,
		ViaCIDRs:     viaCIDRs,
	}
	response, err := r.client.Integrations.UpdateIntegration(input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	applications := parseApplications(response.Applications)
	appType := req.State.Schema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()
	apps, aErr := types.SetValueFrom(ctx, appType, applications)
	if aErr.HasError() {
		resp.Diagnostics.Append(aErr...)
		return
	}
	plan.Application = apps
	newId := types.StringValue(newIDForIntegrationResource(modelName, response.Applications))
	plan.ID = newId
	r.trace(fmt.Sprintf("Updated integration resource: %q", newId))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "delete")
		return
	}

	var state integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName := state.ModelName.ValueString()

	var apps []nestedApplication
	state.Application.ElementsAs(ctx, &apps, false)
	endpoints, _, _, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Provider Error", err.Error())
		return
	}

	// Remove the integration
	err = r.client.Integrations.DestroyIntegration(&juju.IntegrationInput{
		ModelName: modelName,
		Endpoints: endpoints,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}
	r.trace(fmt.Sprintf("Deleted integration resource: %q", state.ID.ValueString()))
}

func handleIntegrationNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if errors.As(err, &juju.NoIntegrationFoundError) {
		// Integration manually removed
		st.RemoveResource(ctx)
		return diag.Diagnostics{}
	}
	var diags diag.Diagnostics
	diags.AddError("Client Error", err.Error())
	return diags
}

func newIDForIntegrationResource(modelName string, apps []juju.Application) string {
	//In order to generate a stable iterable order we sort the endpoints keys by the role value (provider is always first to match `juju status` output)
	//TODO: verify we always get only 2 endpoints and that the role value is consistent
	keys := make([]int, len(apps))
	for k, v := range apps {
		if v.Role == "provider" {
			keys[0] = k
		} else if v.Role == "requirer" {
			keys[1] = k
		}
	}

	id := modelName
	for _, key := range keys {
		ep := apps[key]
		id = fmt.Sprintf("%s:%s:%s", id, ep.Name, ep.Endpoint)
	}

	return id
}

func modelNameAndEndpointsFromID(ID string) (string, string, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	id := strings.Split(ID, ":")
	if len(id) != 5 {
		diags.AddError("Malformed ID",
			fmt.Sprintf("unable to parse model and application name from provided ID: %q", ID))
		return "", "", "", diags
	}
	return id[0], fmt.Sprintf("%v:%v", id[1], id[2]), fmt.Sprintf("%v:%v", id[3], id[4]), diags
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []nestedApplication) (endpoints []string, offer *string, appNames []string, err error) {
	for _, app := range apps {
		name := app.Name.ValueString()
		offerURL := app.OfferURL.ValueString()
		endpoint := app.Endpoint.ValueString()

		//Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		//If the endpoint is specified we pass the format <applicationName>:<endpoint>
		//first check if we have an offer_url, in this case don't return the endpoint
		if offerURL != "" {
			offer = &offerURL
			continue
		}
		if endpoint == "" {
			endpoints = append(endpoints, name)
		} else {
			endpoints = append(endpoints, fmt.Sprintf("%v:%v", name, endpoint))
		}
		// If there is no appname and this is not an offer, we have an app name
		if name != "" && offerURL == "" {
			appNames = append(appNames, name)
		}
	}

	if len(endpoints) == 0 {
		return nil, nil, nil, fmt.Errorf("no endpoints are provided with given applications %v", apps)
	}

	return endpoints, offer, appNames, nil
}

func parseApplications(apps []juju.Application) []nestedApplication {
	applications := make([]nestedApplication, 2)

	for i, app := range apps {
		a := nestedApplication{}

		if app.OfferURL != nil {
			a.OfferURL = types.StringValue(*app.OfferURL)
		} else {
			a.Endpoint = types.StringValue(app.Endpoint)
			a.Name = types.StringValue(app.Name)
		}
		applications[i] = a
	}

	return applications
}

func (r *integrationResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(r.subCtx, LogResourceIntegration, msg, additionalFields...)
}
