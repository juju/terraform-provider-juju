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
	Via         types.String `tfsdk:"via"`
	Application types.Set    `tfsdk:"application"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type integrationResourceModelV0 struct {
	integrationResourceModel

	ModelName types.String `tfsdk:"model"`
}

type integrationResourceModelV1 struct {
	integrationResourceModel

	ModelUUID types.String `tfsdk:"model_uuid"`
}
type nestedApplicationV0 struct {
	Name     types.String `tfsdk:"name"`
	Endpoint types.String `tfsdk:"endpoint"`
	OfferURL types.String `tfsdk:"offer_url"`
}

// nestedApplication represents an element in an Application set of an
// integration resource
type nestedApplication struct {
	Name               types.String `tfsdk:"name"`
	Endpoint           types.String `tfsdk:"endpoint"`
	OfferURL           types.String `tfsdk:"offer_url"`
	OfferingController types.String `tfsdk:"offering_controller"`
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
	var configData integrationResourceModelV1

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
		Version:     1,
		Description: "A resource that represents a Juju Integration.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model to operate in.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
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
							Computed: true,
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
								NewValidatorOfferURL(),
							},
						},
						"offering_controller": schema.StringAttribute{
							Description: "The name of the offering controller where the remote application is hosted. " +
								"This is required when using offer_url to consume an offer from a different controller.",
							Optional: true,
							PlanModifiers: []planmodifier.String{
								// RequiresReplace because the most likely scenario when the name is changed is that it's a different controller,
								// so we need to re-consume the offer.
								// In case it was the same controller with a different name, we will just recreate the integration, which is not
								// harmful.
								stringplanmodifier.RequiresReplace(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName("name"),
								}...),
								stringvalidator.AlsoRequires(path.MatchRelative().AtParent().AtName("offer_url")),
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

	var plan integrationResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelUUID := plan.ModelUUID.ValueString()

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

	// If we have an offer URL, we need to consume it (creating a remote-app) before creating the integration.
	// If the remote-app already exists, we will re-use it (see `ConsumeRemoteOffer` for more details).
	if offer != nil {
		offerResponse, err := r.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelUUID:          modelUUID,
			OfferURL:           offer.url,
			OfferingController: offer.offeringController,
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
		ModelUUID: modelUUID,
		Apps:      appNames,
		Endpoints: endpoints,
		ViaCIDRs:  viaCIDRs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create integration, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("integration created on Juju between %q at %q on model %q", appNames, endpoints, modelUUID))
	isExternal, diags := isExternalOffer(plan.Application)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	parsedApplications, err := parseApplications(response.Applications, isExternal)
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

	id := newIDForIntegrationResource(modelUUID, response.Applications)
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

	var state integrationResourceModelV1

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID, endpointA, endpointB, idErr := modelUUIDAndEndpointsFromID(state.ID.ValueString())
	if idErr.HasError() {
		resp.Diagnostics.Append(idErr...)
		return
	}

	integration := &juju.IntegrationInput{
		ModelUUID: modelUUID,
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

	state.ModelUUID = types.StringValue(modelUUID)
	isExternalOffer, diagErr := isExternalOffer(state.Application)
	if diagErr.HasError() {
		resp.Diagnostics.Append(diagErr...)
		return
	}
	applications, err := parseApplications(response.Applications, isExternalOffer)
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

// isExternalOffer checks if at least one of the applications in the integration
// is from an external controller by inspecting the offering_controller field.
// If at least one application has offering_controller set, it returns true.
func isExternalOffer(appsSet types.Set) (bool, diag.Diagnostics) {
	var apps []nestedApplication
	diags := appsSet.ElementsAs(context.Background(), &apps, false)
	if diags.HasError() {
		return false, diags
	}
	for _, app := range apps {
		if !app.OfferingController.IsNull() {
			return true, nil
		}
	}
	return false, nil
}

// Update is a no-op, as all fields force replacement.
func (r *integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete removes the integration and intentionally avoids deleting any consumed offers
// in case multiple apps are using the same consumed offer (remote app).
func (r *integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "delete")
		return
	}

	var state integrationResourceModelV1
	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelUUID, endpointA, endpointB, idErr := modelUUIDAndEndpointsFromID(state.ID.ValueString())
	if idErr.HasError() {
		resp.Diagnostics.Append(idErr...)
		return
	}
	endpoints := []string{endpointA, endpointB}
	err := r.client.Integrations.DestroyIntegration(&juju.IntegrationInput{
		ModelUUID: modelUUID,
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
				ModelUUID: modelUUID,
				Endpoints: endpoints,
			},
			ExpectedErr:    juju.IntegrationNotFoundError,
			RetryAllErrors: true,
		},
	)
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete integration deletion (endpoints %v) in model %q: %v\n", endpoints, modelUUID, err)
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
}

// UpgradeState upgrades the state of the integration resource.
// This is used to handle changes in the resource schema between versions.
// V0->V2: The model name is replaced with the model UUID and offering_controller field is added.
func (r *integrationResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: integrationV0Schema(),
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				integrationV0 := integrationResourceModelV0{}
				resp.Diagnostics.Append(req.State.Get(ctx, &integrationV0)...)

				if resp.Diagnostics.HasError() {
					return
				}

				modelUUID, err := r.client.Models.ModelUUID(integrationV0.ModelName.ValueString(), "")
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model UUID for model %q, got error: %s", integrationV0.ModelName.ValueString(), err))
					return
				}

				newID := strings.Replace(integrationV0.ID.ValueString(), integrationV0.ModelName.ValueString(), modelUUID, 1)

				// Parse old applications and reconstruct with new schema including offering_controller field
				var oldApps []nestedApplicationV0
				resp.Diagnostics.Append(integrationV0.Application.ElementsAs(ctx, &oldApps, false)...)
				if resp.Diagnostics.HasError() {
					return
				}

				// Reconstruct applications with the new structure
				upgradedApps := make([]nestedApplication, len(oldApps))
				for i, app := range oldApps {
					upgradedApps[i] = nestedApplication{
						Name:               app.Name,
						Endpoint:           app.Endpoint,
						OfferURL:           app.OfferURL,
						OfferingController: types.StringNull(),
					}
				}

				// Create the new Application set with the current schema type
				appsType := resp.State.Schema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()
				upgradedAppsSet, errDiag := types.SetValueFrom(ctx, appsType, upgradedApps)
				resp.Diagnostics.Append(errDiag...)
				if resp.Diagnostics.HasError() {
					return
				}

				upgradedStateData := integrationResourceModelV1{
					integrationResourceModel: integrationResourceModel{
						Via:         integrationV0.Via,
						ID:          types.StringValue(newID),
						Application: upgradedAppsSet,
					},
					ModelUUID: types.StringValue(modelUUID),
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
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

func newIDForIntegrationResource(modelUUID string, apps []juju.Application) string {
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

	id := modelUUID
	for _, key := range keys {
		ep := apps[key]
		id = fmt.Sprintf("%s:%s:%s", id, ep.Name, ep.Endpoint)
	}

	return id
}

func modelUUIDAndEndpointsFromID(ID string) (string, string, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	id := strings.Split(ID, ":")
	if len(id) != 5 {
		diags.AddError("Malformed ID",
			fmt.Sprintf("unable to parse model UUID and application name from provided ID: %q", ID))
		return "", "", "", diags
	}
	return id[0], fmt.Sprintf("%v:%v", id[1], id[2]), fmt.Sprintf("%v:%v", id[3], id[4]), diags
}

type offer struct {
	url                string
	endpoint           string
	offeringController string
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []nestedApplication) (endpoints []string, of *offer, appNames []string, err error) {
	for _, app := range apps {
		name := app.Name.ValueString()
		offerURL := app.OfferURL.ValueString()
		endpoint := app.Endpoint.ValueString()
		offeringController := app.OfferingController.ValueString()

		// Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		// If the endpoint is specified we pass the format <applicationName>:<endpoint>
		// first check if we have an offer_url, in this case don't return the endpoint
		if offerURL != "" {
			of = &offer{
				url:                offerURL,
				endpoint:           endpoint,
				offeringController: offeringController,
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

// parseApplications converts Juju applications into nestedApplication structs for the resource state
// If isExternalController is true, it means at least one of the applications is from an external controller,
// so we need to include the offering_controller field when parsing the applications.
// This is required because if an offer is created via the CLI, the offer URL contains the external controller even
// if the remote app is on the same controller.
func parseApplications(apps []juju.Application, isExternalController bool) ([]nestedApplication, error) {
	applications := make([]nestedApplication, 2)

	for i, app := range apps {
		a := nestedApplication{}

		if app.OfferURL != nil {
			url, err := crossmodel.ParseOfferURL(*app.OfferURL)
			if err != nil {
				return nil, fmt.Errorf("failed to parse offer URL %q: %w", *app.OfferURL, err)
			}
			a.OfferURL = types.StringValue(url.AsLocal().String())
			if isExternalController {
				if url.Source == "" {
					return nil, fmt.Errorf("offering controller is required for offer URL %q", *app.OfferURL)
				}
				a.OfferingController = types.StringValue(url.Source)
			}
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

func integrationV0Schema() *schema.Schema {
	return &schema.Schema{
		Description: "A resource that represents a Juju Integration.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Required: true,
			},
			"via": schema.StringAttribute{
				Optional: true,
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"application": schema.SetNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Optional: true,
						},
						"endpoint": schema.StringAttribute{
							Optional: true,
							Computed: true,
						},
						"offer_url": schema.StringAttribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}
