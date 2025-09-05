// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
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
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/core/crossmodel"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
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
	config juju.Config

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

	provider, ok := req.ProviderData.(*juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *juju.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = provider.Client
	r.config = provider.Config
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
							Description: "The name of the application. This attribute may not be used at the" +
								" same time as the offer_url.",
							Optional: true,
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName("offer_url"),
								}...),
							},
						},
						"endpoint": schema.StringAttribute{
							Description: "The endpoint name. This attribute may not be used at the" +
								" same time as the offer_url.",
							Optional: true,
							Computed: true,
						},
						"offer_url": schema.StringAttribute{
							Description: "The URL of a remote application. This attribute may not be used at the" +
								" same time as name and endpoint.",
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName("name"),
								}...),
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

	endpoints, offer, appNames, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse endpoints, got error: %s", err))
		return
	}

	// If we have an offer URL, we need to consume it before creating the integration.
	if offer != nil {
		offerResponse, err := r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelName: modelName,
			OfferURL:  offer.url,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to consume remote offer, got error: %s", err))
			return
		}
		r.trace(fmt.Sprintf("remote offer created : %q", *offer))
		// If the offer has a SAASName, we append it to the endpoints list
		// If the endpoint is not empty, we append it in the format <SAASName>:<endpoint>
		// If the endpoint is empty, we append just the SAASName and Juju will infer the endpoint.
		if offerResponse.SAASName != "" {
			if offer.endpoint != "" {
				endpoints = append(endpoints, fmt.Sprintf("%s:%s", offerResponse.SAASName, offer.endpoint))
			} else {
				endpoints = append(endpoints, offerResponse.SAASName)
			}
		}
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

	parsedApplications, err := parseApplications(response.Applications)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse applications, got error: %s", err))
		return
	}

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

	applications, err := parseApplications(response.Applications)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse applications, got error: %s", err))
		return
	}

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
	var oldOffer, offer *offer
	var err error

	if !plan.Application.Equal(state.Application) {
		var oldApps []nestedApplication
		state.Application.ElementsAs(ctx, &oldApps, false)
		oldEndpoints, oldOffer, _, err = parseEndpoints(oldApps)
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}

		var newApps []nestedApplication
		plan.Application.ElementsAs(ctx, &newApps, false)
		endpoints, offer, _, err = parseEndpoints(newApps)
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}
	}

	var offerResponse *juju.ConsumeRemoteOfferResponse
	// check if the offer url has been deleted or the URL has been changed.
	if oldOffer != nil && (offer == nil || offer.url != oldOffer.url) {
		// Destroy old remote offer. This is not automatically handled by the `requires_replace` logic,
		// because it is a special case where the offer URL has been specified in an integration resource.
		errs := r.client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
			ModelName: modelName,
			OfferURL:  oldOffer.url,
		})
		if len(errs) > 0 {
			for _, v := range errs {
				resp.Diagnostics.AddError("Client Error", v.Error())
			}
			return
		}
		r.trace(fmt.Sprintf("removed offer on Juju: %q", oldOffer.url))
	}
	// check if the offer url has been added or the URL has been changed.
	if offer != nil && (oldOffer == nil || offer.url != oldOffer.url) {
		offerResponse, err = r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelName: modelName,
			OfferURL:  offer.url,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}
		// If the offer has a SAASName, we append it to the endpoints list
		// If the endpoint is not empty, we append it in the format <SAASName>:<endpoint>
		// If the endpoint is empty, we append just the SAASName and Juju will infer the endpoint.
		if offerResponse.SAASName != "" {
			if offer.endpoint != "" {
				endpoints = append(endpoints, fmt.Sprintf("%s:%s", offerResponse.SAASName, offer.endpoint))
			} else {
				endpoints = append(endpoints, offerResponse.SAASName)
			}
		}
		r.trace(fmt.Sprintf("added offer on Juju: %q", offer.url))
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

	applications, err := parseApplications(response.Applications)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse applications, got error: %s", err))
		return
	}

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
	modelName, endpointA, endpointB, idErr := modelNameAndEndpointsFromID(state.ID.ValueString())
	if idErr.HasError() {
		resp.Diagnostics.Append(idErr...)
		return
	}
	endpoints := []string{endpointA, endpointB}
	err := r.client.Integrations.DestroyIntegration(&juju.IntegrationInput{
		ModelName: modelName,
		Endpoints: endpoints,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	err = wait.WaitForError(
		wait.WaitForErrorCfg[*juju.IntegrationInput, *juju.ReadIntegrationResponse]{
			Context: ctx,
			GetData: r.client.Integrations.ReadIntegration,
			Input: &juju.IntegrationInput{
				ModelName: modelName,
				Endpoints: endpoints,
			},
			ErrorToWait:    juju.IntegrationNotFoundError,
			NonFatalErrors: []error{juju.ConnectionRefusedError, juju.RetryReadError},
		},
	)
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete integration %v for model %s deletion due to error %v, there might be dangling resources.\n"+
			"Make sure to manually delete them.", endpoints, modelName, err)
		if r.config.SkipFailedDeletion {
			resp.Diagnostics.AddWarning(
				errSummary,
				errDetail,
			)
		} else {
			resp.Diagnostics.AddError(
				errSummary,
				errDetail,
			)
		}
		return
	}
	r.trace(fmt.Sprintf("Deleted integration resource: %q", state.ID.ValueString()))
}

func handleIntegrationNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if errors.Is(err, juju.IntegrationNotFoundError) {
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

type offer struct {
	url      string
	endpoint string
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []nestedApplication) (endpoints []string, of *offer, appNames []string, err error) {
	for _, app := range apps {
		name := app.Name.ValueString()
		offerURL := app.OfferURL.ValueString()
		endpoint := app.Endpoint.ValueString()

		//Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		//If the endpoint is specified we pass the format <applicationName>:<endpoint>
		//first check if we have an offer_url, in this case don't return the endpoint
		if offerURL != "" {
			of = &offer{
				url:      offerURL,
				endpoint: endpoint,
			}
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

	return endpoints, of, appNames, nil
}

func parseApplications(apps []juju.Application) ([]nestedApplication, error) {
	applications := make([]nestedApplication, 2)

	for i, app := range apps {
		a := nestedApplication{}

		if app.OfferURL != nil {
			url, err := cleanOfferURL(*app.OfferURL)
			if err != nil {
				return nil, err
			}
			a.OfferURL = types.StringValue(url)
			a.Endpoint = types.StringValue(app.Endpoint)
		} else {
			a.Endpoint = types.StringValue(app.Endpoint)
			a.Name = types.StringValue(app.Name)
		}
		applications[i] = a
	}

	return applications, nil
}

// cleanOfferURL removes the source field from the offer URL.
// The source represents the source controller of the offer.
//
// The Juju CLI sets the source field on the offer URL string when the offer is consumed.
// The Terraform provider leaves this field empty since it is does not support
// cross-controller relations.
//
// Until that changes, we clean the URL to assist in scenarios where an offer URL
// has the source field set.
func cleanOfferURL(offerURL string) (string, error) {
	url, err := crossmodel.ParseOfferURL(offerURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse offer URL %q: %w", offerURL, err)
	}
	url.Source = ""
	return url.String(), nil
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
