package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	frameworkdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkResSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &integrationResource{}
var _ resource.ResourceWithConfigure = &integrationResource{}
var _ resource.ResourceWithImportState = &integrationResource{}

func NewIntegrationResource() resource.Resource {
	return &integrationResource{}
}

type integrationResource struct {
	client *juju.Client
}

func (i integrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (i integrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	i.client = client
}

func (i integrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

func (i integrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = frameworkResSchema.Schema{
		Description: "A resource that represents a Juju Integration.",
		Attributes: map[string]frameworkResSchema.Attribute{
			"model": frameworkResSchema.StringAttribute{
				Description: "The name of the model to operate in.",
				Required:    true,
			},
			"via": frameworkResSchema.StringAttribute{
				Description: "A comma separated list of CIDRs for outbound traffic.",
				Optional:    true,
			},
			"application": frameworkResSchema.SetNestedAttribute{
				Description: "The two applications to integrate.",
				NestedObject: frameworkResSchema.NestedAttributeObject{
					Attributes: map[string]frameworkResSchema.Attribute{
						"name": frameworkResSchema.StringAttribute{
							Description: "The name of the application.",
							Optional:    true,
						},
						"endpoint": frameworkResSchema.StringAttribute{
							Description: "The endpoint name.",
							Optional:    true,
							Computed:    true,
						},
						"offer_url": frameworkResSchema.StringAttribute{
							Description: "The URL of a remote application.",
							Optional:    true,
							// Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
			"id": frameworkResSchema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (i integrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if i.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "ssh_key", "create")
		return
	}

	var data integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelName := data.ModelName.ValueString()
	modelUUID, err := i.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve model UUID, got error: %s", err))
		return
	}

	var apps []map[string]string
	resp.Diagnostics.Append(data.Application.ElementsAs(ctx, &apps, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoints, offerURL, appNames, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse endpoints, got error: %s", err))
		return
	}
	if len(endpoints) == 0 {
		resp.Diagnostics.AddError("Client Error", "please provide at least one local application")
		return
	}
	var offerResponse = &juju.ConsumeRemoteOfferResponse{}
	if offerURL != nil {
		offerResponse, err = i.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelUUID: modelUUID,
			OfferURL:  *offerURL,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to consume remote offer, got error: %s", err))
			return
		}
	}

	if offerResponse.SAASName != "" {
		endpoints = append(endpoints, offerResponse.SAASName)
	}

	viaCIDRs := data.Via.ValueString()
	response, err := i.client.Integrations.CreateIntegration(&juju.IntegrationInput{
		ModelUUID: modelUUID,
		Apps:      appNames,
		Endpoints: endpoints,
		ViaCIDRs:  viaCIDRs,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create integration, got error: %s", err))
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("created integration resource between apps: %q", appNames))

	parsedApplications := parseApplications(response.Applications)

	blockAttributeType := map[string]attr.Type{
		"name":      types.StringType,
		"endpoint":  types.StringType,
		"offer_url": types.StringType,
	}

	appsType := types.ObjectType{AttrTypes: blockAttributeType}

	parsedApps, errDiag := types.SetValueFrom(ctx, appsType, parsedApplications)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Application = parsedApps

	id := newIDForIntegrationResource(modelName, response.Applications)
	data.ID = types.StringValue(id)

	// Write the state data into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (i integrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if i.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "integration", "read")
		return
	}

	var plan integrationResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, provApp, provEndP, reqApp, reqEndP, idErr := modelIntegrationNameAndEndpointsFromID(plan.ID.ValueString())
	if idErr.HasError() {
		resp.Diagnostics.Append(idErr...)
		return
	}

	modelUUID, err := i.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model uuid, got error: %s", err))
		return
	}

	integration := &juju.IntegrationInput{
		ModelUUID: modelUUID,
		Endpoints: []string{
			fmt.Sprintf("%v:%v", provApp, provEndP),
			fmt.Sprintf("%v:%v", reqApp, reqEndP),
		},
	}

	response, err := i.client.Integrations.ReadIntegration(integration)
	if err != nil {
		resp.Diagnostics.Append(handleIntegrationNotFoundError(ctx, err, &resp.State)...)
		return
	}
	tflog.Trace(ctx, fmt.Sprintf("read integration resource %q", plan.ID.ValueString()))

	plan.ModelName = types.StringValue(modelName)

	applications := parseApplications(response.Applications)
	appType := req.State.Schema.GetAttributes()["application"].(frameworkResSchema.SetNestedAttribute).NestedObject.Type()
	apps, aErr := types.SetValueFrom(ctx, appType, applications)
	if aErr.HasError() {
		resp.Diagnostics.Append(aErr...)
		return
	}
	plan.Application = apps

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (i integrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if i.client == nil {
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
	modelUUID, err := i.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model uuid, got error: %s", err))
		return
	}

	var oldEndpoints, endpoints []string
	var oldOfferURL, offerURL *string

	if !plan.Application.Equal(state.Application) {
		var oldApps []map[string]string
		state.Application.ElementsAs(ctx, &oldApps, false)
		oldEndpoints, oldOfferURL, _, err = parseEndpoints(oldApps)
		if err != nil {
			resp.Diagnostics.AddError("Provider Error", err.Error())
			return
		}

		var newApps []map[string]string
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
			errs := i.client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
				ModelUUID: modelUUID,
				OfferURL:  *oldOfferURL,
			})
			if len(errs) > 0 {
				for _, v := range errs {
					resp.Diagnostics.AddError("Client Error", v.Error())
				}
				return
			}
		}
		if offerURL != nil {
			offerResponse, err = i.client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
				ModelUUID: modelUUID,
				OfferURL:  *offerURL,
			})
			if err != nil {
				resp.Diagnostics.AddError("Client Error", err.Error())
				return
			}
			endpoints = append(endpoints, offerResponse.SAASName)
		}
	}

	viaCIDRs := plan.Via.ValueString()
	input := &juju.UpdateIntegrationInput{
		ModelUUID:    modelUUID,
		ID:           plan.ID.ValueString(),
		Endpoints:    endpoints,
		OldEndpoints: oldEndpoints,
		ViaCIDRs:     viaCIDRs,
	}
	response, err := i.client.Integrations.UpdateIntegration(input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	applications := parseApplications(response.Applications)
	appType := req.State.Schema.GetAttributes()["application"].(frameworkResSchema.SetNestedAttribute).NestedObject.Type()
	apps, aErr := types.SetValueFrom(ctx, appType, applications)
	if aErr.HasError() {
		resp.Diagnostics.Append(aErr...)
		return
	}
	plan.Application = apps
	plan.ID = types.StringValue(newIDForIntegrationResource(modelName, response.Applications))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (i integrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if i.client == nil {
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
	modelUUID, err := i.client.Models.ResolveModelUUID(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model uuid, got error: %s", err))
		return
	}

	var apps []map[string]string
	state.Application.ElementsAs(ctx, &apps, false)
	endpoints, offer, _, err := parseEndpoints(apps)
	if err != nil {
		resp.Diagnostics.AddError("Provider Error", err.Error())
		return
	}

	//If one of the endpoints is an offer then we need to remove the remote offer rather than destroying the integration
	if offer != nil {
		errs := i.client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
			ModelUUID: modelUUID,
			OfferURL:  *offer,
		})
		if len(errs) > 0 {
			for _, v := range errs {
				resp.Diagnostics.AddError("Client Error", v.Error())
			}
			return
		}
	} else {
		err = i.client.Integrations.DestroyIntegration(&juju.IntegrationInput{
			ModelUUID: modelUUID,
			Endpoints: endpoints,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", err.Error())
			return
		}
	}
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

func IsIntegrationNotFound(err error) bool {
	return strings.Contains(err.Error(), "no integrations exist")
}

func handleIntegrationNotFoundError(ctx context.Context, err error, st *tfsdk.State) frameworkdiag.Diagnostics {
	if IsIntegrationNotFound(err) {
		// Integration manually removed
		st.RemoveResource(ctx)
		return frameworkdiag.Diagnostics{}
	}
	var diags frameworkdiag.Diagnostics
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

func modelIntegrationNameAndEndpointsFromID(ID string) (string, string, string, string, string, frameworkdiag.Diagnostics) {
	var diags frameworkdiag.Diagnostics
	id := strings.Split(ID, ":")
	if len(id) != 5 {
		diags.AddError("Malformed ID",
			fmt.Sprintf("unable to parse model and application name from provided ID: %q", ID))
		return "", "", "", "", "", diags
	}
	return id[0], id[1], id[2], id[3], id[4], diags
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []map[string]string) (endpoints []string, offer *string, appNames []string, err error) {
	for _, app := range apps {
		if app == nil {
			return nil, nil, nil, fmt.Errorf("you must provide a non-empty name for each application in an integration")
		}
		name := app["name"]
		offerURL := app["offer_url"]
		endpoint := app["endpoint"]

		if name == "" && offerURL == "" {
			return nil, nil, nil, fmt.Errorf("you must provide one of \"name\" or \"offer_url\"")
		}

		if name != "" && offerURL != "" {
			return nil, nil, nil, fmt.Errorf("you must only provider one of \"name\" or \"offer_url\" and not both")
		}

		if offerURL != "" && endpoint != "" {
			return nil, nil, nil, fmt.Errorf("\"offer_url\" cannot be provided with \"endpoint\"")
		}

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

	return endpoints, offer, appNames, nil
}

func parseApplications(apps []juju.Application) []map[string]string {
	applications := make([]map[string]string, 0, 2)

	for _, app := range apps {
		a := make(map[string]string)

		if app.OfferURL != nil {
			a["offer_url"] = *app.OfferURL
			a["endpoint"] = ""
			a["name"] = ""
		} else {
			a["endpoint"] = app.Endpoint
			a["name"] = app.Name
			a["offer_url"] = ""
		}
		applications = append(applications, a)
	}

	return applications
}
