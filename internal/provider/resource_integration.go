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
	"github.com/juju/names/v5"

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

type integrationApps []nestedApplication

// nestedApplication represents an element in an Application set of an
// integration resource
type nestedApplication struct {
	Name     types.String `tfsdk:"name"`
	Endpoint types.String `tfsdk:"endpoint"`
	OfferURL types.String `tfsdk:"offer_url"`
	// AppSuffix is used when relating to an offer URL
	// in order to create a unique saas app and avoid relating
	// multiple local applications with the same offer.
	AppSuffix types.String `tfsdk:"app_suffix"`
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
	resp.Diagnostics.Append(configData.Application.ElementsAs(ctx, &apps, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"via": schema.StringAttribute{
				Description: "A comma separated list of CIDRs for outbound traffic.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
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
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName("offer_url"),
								}...),
							},
						},
						"endpoint": schema.StringAttribute{
							Description: "The endpoint name. This attribute may not be used at the" +
								" same time as the offer_url.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplace(),
							},
							Optional: true,
							Computed: true,
						},
						"offer_url": schema.StringAttribute{
							Description: "The URL of a remote application. This attribute may not be used at the" +
								" same time as name and endpoint.",
							Optional: true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName("name"),
								}...),
							},
						},
						"app_suffix": schema.StringAttribute{
							Description: "A suffix appended to the SAAS application created in cross-model relations." +
								" This is computed by the provider and avoids relating multiple local apps to a single remote app.",
							Computed: true,
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

	var apps integrationApps
	resp.Diagnostics.Append(plan.Application.ElementsAs(ctx, &apps, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apps = createRemoteAppSuffix(apps)

	endpoints, offer, appNames, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse endpoints, got error: %s", err))
		return
	}

	// If we have an offer URL, we need to consume it before creating the integration.
	if offer != nil {
		remoteAppName, err := offer.remoteAppName()
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}
		offerResponse, err := r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelName:      modelName,
			OfferURL:       offer.url,
			RemoteAppAlias: remoteAppName,
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

	parsedApplications, err := apps.mergeJujuData(response.Applications)
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

	var apps integrationApps
	resp.Diagnostics.Append(state.Application.ElementsAs(ctx, &apps, false)...)
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

	appWithJujuData, err := apps.mergeJujuData(response.Applications)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to merge Juju data, got error: %s", err))
		return
	}

	appType := req.State.Schema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()
	appsSet, aErr := types.SetValueFrom(ctx, appType, appWithJujuData)
	if aErr.HasError() {
		resp.Diagnostics.Append(aErr...)
		return
	}

	state.Application = appsSet

	r.trace(fmt.Sprintf("read integration resource: %v", state.ID.ValueString()))
	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op, as all fields force replacement.
func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete removes the integration and, if it was cross-model, the consumed offer.
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
			ExpectedErr:    juju.IntegrationNotFoundError,
			RetryAllErrors: true,
		},
	)
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete integration deletion (endpoints %v) in model %q: %v\n", endpoints, modelName, err)
		if r.config.SkipFailedDeletion {
			resp.Diagnostics.AddWarning(
				errSummary,
				errDetail+"There might be dangling resources requiring manual intervion.\n",
			)
		} else {
			resp.Diagnostics.AddError(
				errSummary,
				errDetail,
			)
			return
		}
	}

	r.trace(fmt.Sprintf("Deleted integration resource: %q", state.ID.ValueString()))

	var offer *offer
	var apps []nestedApplication
	resp.Diagnostics.Append(state.Application.ElementsAs(ctx, &apps, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	_, offer, _, err = parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Provider Error", err.Error())
		return
	}

	// check if the integration had consumed an offer.
	if offer == nil {
		return
	}

	// Destroy consumed offer.
	remoteAppName, err := offer.remoteAppName()
	if err != nil {
		resp.Diagnostics.AddError("Provider Error", err.Error())
		return
	}

	err = r.client.Offers.RemoveRemoteApp(&juju.RemoveRemoteAppInput{
		ModelName:     modelName,
		RemoteAppName: remoteAppName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	err = wait.WaitForError(
		wait.WaitForErrorCfg[*juju.ReadRemoteAppInput, *juju.ReadRemoteAppResponse]{
			Context: ctx,
			GetData: r.client.Offers.ReadRemoteApp,
			Input: &juju.ReadRemoteAppInput{
				ModelName:     modelName,
				RemoteAppName: remoteAppName,
			},
			ExpectedErr:    juju.RemoteAppNotFoundError,
			RetryAllErrors: true,
		},
	)
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete remote-app %q deletion in model %q: %v\n", remoteAppName, modelName, err)
		if r.config.SkipFailedDeletion {
			resp.Diagnostics.AddWarning(
				errSummary,
				errDetail+"There might be dangling resources requiring manual intervion.\n",
			)
		} else {
			resp.Diagnostics.AddError(
				errSummary,
				errDetail,
			)
			return
		}
	}

	r.trace(fmt.Sprintf("removed remote app %q", remoteAppName))
}

// createRemoteAppSuffix checks if one of the applications is a cross-model offer
// and if so, returns a copy of the apps with the AppSUffix field populated.
// This should only be called during Create.
//
// As background, the Juju API allows a user to consume an offer with a
// specified alias. The Terraform provider does not expose this to the user,
// and instead generates a unique name for each consumed offer.
//
// The suffix has the format "-<local-app-name>-<local-endpoint>" and is stored separately
// for backwards compatibility with old versions of the provider where it may be empty.
// This function returns a new slice of applications with the suffix added
// to the offer application.
func createRemoteAppSuffix(apps []nestedApplication) []nestedApplication {
	var localApp, offer nestedApplication
	var crossModel bool
	for _, app := range apps {
		if app.OfferURL.ValueString() != "" {
			crossModel = true
			offer = app
			continue
		}
		localApp = app
	}
	if !crossModel {
		return apps
	}
	suffix := fmt.Sprintf("-%s-%s", localApp.Name.ValueString(), localApp.Endpoint.ValueString())
	offer.AppSuffix = types.StringValue(suffix)
	return []nestedApplication{localApp, offer}
}

// remoteAppName constructs the remote application name from
// the offer URL and the remote-app suffix created when the
// offer was consumed.
//
// The suffix may be empty if the offer was consumed using
// an older version of the provider.
func (offer offer) remoteAppName() (string, error) {
	url, err := crossmodel.ParseOfferURL(offer.url)
	if err != nil {
		return "", fmt.Errorf("failed to parse offer URL %q: %w", offer.url, err)
	}
	remoteAppName := url.ApplicationName + offer.remoteAppSuffix
	if !names.IsValidApplication(remoteAppName) {
		return "", fmt.Errorf("the constructed remote application name %q is not valid", remoteAppName)
	}
	return remoteAppName, nil
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
	url             string
	endpoint        string
	remoteAppSuffix string
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []nestedApplication) (endpoints []string, of *offer, appNames []string, err error) {
	for _, app := range apps {
		name := app.Name.ValueString()
		offerURL := app.OfferURL.ValueString()
		endpoint := app.Endpoint.ValueString()
		appSuffix := app.AppSuffix.ValueString()

		//Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		//If the endpoint is specified we pass the format <applicationName>:<endpoint>
		//first check if we have an offer_url, in this case don't return the endpoint
		if offerURL != "" {
			of = &offer{
				url:             offerURL,
				endpoint:        endpoint,
				remoteAppSuffix: appSuffix,
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

func (stateApps integrationApps) mergeJujuData(apps []juju.Application) ([]nestedApplication, error) {
	switch len(stateApps) {
	case 0:
		// empty state, normal during an import.
	case 2:
		// expected case when we have data.
	default:
		return nil, fmt.Errorf("expected either 2 or 0 applications in relation, got %d", len(stateApps))
	}

	applications, err := parseApplications(apps)
	if err != nil {
		return nil, err
	}

	// Now we match the offer URLs to copy over the app suffix.
	// This is some ugly code to map between 2 sets where we combine the
	// data from Juju which inferred the endpoints involved in the relation,
	// with our local data which has the app suffix for cross-model relations.
	for i, app := range applications {
		if app.OfferURL.ValueString() != "" {
			for _, localApp := range stateApps {
				if localApp.OfferURL.ValueString() == app.OfferURL.ValueString() {
					app.AppSuffix = localApp.AppSuffix
					applications[i] = app
				}
			}
		}
	}

	return applications, nil
}

func parseApplications(apps []juju.Application) ([]nestedApplication, error) {
	applications := make([]nestedApplication, 2)

	for i, app := range apps {
		a := nestedApplication{}

		if app.OfferURL != nil {
			url := *app.OfferURL
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

func (r *integrationResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(r.subCtx, LogResourceIntegration, msg, additionalFields...)
}
