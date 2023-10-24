// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/core/constraints"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

const (
	CharmKey     = "charm"
	CidrsKey     = "cidrs"
	ConfigKey    = "config"
	EndpointsKey = "endpoints"
	ExposeKey    = "expose"
	SpacesKey    = "spaces"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &applicationResource{}
var _ resource.ResourceWithConfigure = &applicationResource{}
var _ resource.ResourceWithImportState = &applicationResource{}

func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// applicationResourceModel describes the application data model.
// tfsdk must match user resource schema attribute names.
type applicationResourceModel struct {
	ApplicationName types.String `tfsdk:"name"`
	Charm           types.List   `tfsdk:"charm"`
	Config          types.Map    `tfsdk:"config"`
	Constraints     types.String `tfsdk:"constraints"`
	Expose          types.List   `tfsdk:"expose"`
	ModelName       types.String `tfsdk:"model"`
	Placement       types.String `tfsdk:"placement"`
	// TODO - remove Principal when we version the schema
	// and remove deprecated elements. Once we create upgrade
	// functionality it can be removed from the structure.
	Principal types.Bool  `tfsdk:"principal"`
	Trust     types.Bool  `tfsdk:"trust"`
	UnitCount types.Int64 `tfsdk:"units"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (r *applicationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (r *applicationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	// Create the local logging subsystem here, using the TF context when creating it.
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceApplication)
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a single Juju application deployment from a charm. Deployment of bundles" +
			" is not supported.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "A custom name for the application deployment. If empty, uses the charm's name.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"model": schema.StringAttribute{
				Description: "The name of the model where the application is to be deployed.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
			},
			"units": schema.Int64Attribute{
				Description: "The number of application units to deploy for the charm.",
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(int64(1)),
			},
			ConfigKey: schema.MapAttribute{
				Description: "Application specific configuration. Must evaluate to a string, integer or boolean.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"constraints": schema.StringAttribute{
				Description: "Constraints imposed on this application.",
				Optional:    true,
				// Set as "computed" to pre-populate and preserve any implicit constraints
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"trust": schema.BoolAttribute{
				Description: "Set the trust for the application.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"placement": schema.StringAttribute{
				Description: "Specify the target location for the application's units",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"principal": schema.BoolAttribute{
				Description: "Whether this is a Principal application",
				Computed:    true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
				DeprecationMessage: "Principal is computed only and not needed. This attribute will be removed in the next major version of the provider.",
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			CharmKey: schema.ListNestedBlock{
				Description: "The name of the charm to be installed from Charmhub.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required:    true,
							Description: "The name of the charm",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.RequiresReplaceIfConfigured(),
							},
						},
						"channel": schema.StringAttribute{
							Description: "The channel to use when deploying a charm. Specified as \\<track>/\\<risk>/\\<branch>.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"revision": schema.Int64Attribute{
							Description: "The revision of the charm to deploy.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						SeriesKey: schema.StringAttribute{
							Description: "The series on which to deploy.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(BaseKey),
								}...),
							},
							DeprecationMessage: "Configure base instead. This attribute will be removed in the next major version of the provider.",
						},
						BaseKey: schema.StringAttribute{
							Description: "The operating system on which to deploy. E.g. ubuntu@22.04.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
							Validators: []validator.String{
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(SeriesKey),
								}...),
								stringIsBaseValidator{},
							},
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
					listvalidator.IsRequired(),
				},
			},
			ExposeKey: schema.ListNestedBlock{
				Description: "Makes an application publicly available over the network",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						EndpointsKey: schema.StringAttribute{
							Description: "Expose only the ports that charms have opened for this comma-delimited list of endpoints",
							Optional:    true,
						},
						SpacesKey: schema.StringAttribute{
							Description: "A comma-delimited list of spaces that should be able to access the application ports once exposed.",
							Optional:    true,
						},
						CidrsKey: schema.StringAttribute{
							Description: "A comma-delimited list of CIDRs that should be able to access the application ports once exposed.",
							Optional:    true,
						},
					},
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
		},
	}
}

// nestedCharm represents the single element of the charm ListNestedBlock
// of the in the application resource schema
type nestedCharm struct {
	Name     types.String `tfsdk:"name"`
	Channel  types.String `tfsdk:"channel"`
	Revision types.Int64  `tfsdk:"revision"`
	Base     types.String `tfsdk:"base"`
	Series   types.String `tfsdk:"series"`
}

// nestedExpose represents the single element of expose ListNestedBlock
// of the in the application resource schema
type nestedExpose struct {
	Endpoints types.String `tfsdk:"endpoints"`
	Spaces    types.String `tfsdk:"spaces"`
	Cidrs     types.String `tfsdk:"cidrs"`
}

func (n nestedExpose) transformToMapStringInterface() map[string]interface{} {
	// An empty map is equivalent to `juju expose` with no
	// endpoints, cidrs nor spaces
	expose := make(map[string]interface{})
	if val := n.Endpoints.ValueString(); val != "" {
		expose[EndpointsKey] = val
	}
	if val := n.Spaces.ValueString(); val != "" {
		expose[SpacesKey] = val
	}
	if val := n.Cidrs.ValueString(); val != "" {
		expose[CidrsKey] = val
	}
	return expose
}

func parseNestedExpose(value map[string]interface{}) nestedExpose {
	// an empty expose structure, indicates exposure
	// the values are optional.
	resp := nestedExpose{}
	if cidrs, ok := value[CidrsKey]; ok && cidrs != "" {
		resp.Cidrs = types.StringValue(cidrs.(string))
	}
	if endpoints, ok := value[EndpointsKey]; ok && endpoints != "" {
		resp.Endpoints = types.StringValue(endpoints.(string))
	}
	if spaces, ok := value[SpacesKey]; ok && spaces != "" {
		resp.Spaces = types.StringValue(spaces.(string))
	}
	return resp
}

// Create is called when the provider must create a new resource. Config
// and planned state values should be read from the
// CreateRequest and new state values set on the CreateResponse.
func (r *applicationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "application", "create")
		return
	}

	var plan applicationResourceModel

	// Read Terraform plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Create", applicationResourceModelForLogging(ctx, &plan))

	charms := []nestedCharm{}
	resp.Diagnostics.Append(plan.Charm.ElementsAs(ctx, &charms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	planCharm := charms[0]
	charmName := planCharm.Name.ValueString()
	channel := "stable"
	if !planCharm.Channel.IsUnknown() {
		channel = planCharm.Channel.ValueString()
	}
	revision := -1
	if !planCharm.Revision.IsUnknown() {
		revision = int(planCharm.Revision.ValueInt64())
	}

	// TODO: investigate using map[string]string here and let
	// terraform do the conversion, will help in CreateApplication.
	configField := map[string]string{}
	resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &configField, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the plan has an empty expose block, that has meaning.
	// It's equivalent to using the expose flag on the juju cli.
	// Be sure to understand if the expose block exists or not.
	// Then to understand if any of the contained values exist.
	var expose map[string]interface{} = nil
	if !plan.Expose.IsNull() {
		var exposeSlice []nestedExpose
		resp.Diagnostics.Append(plan.Expose.ElementsAs(ctx, &exposeSlice, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		r.trace("Creating application, expose values", map[string]interface{}{"exposeSlice": exposeSlice})
		if len(exposeSlice) == 1 {
			expose = exposeSlice[0].transformToMapStringInterface()
		}
	}

	var parsedConstraints = constraints.Value{}
	if plan.Constraints.ValueString() != "" {
		var err error
		parsedConstraints, err = constraints.Parse(plan.Constraints.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Input Error", fmt.Sprintf("Unable to parse constraints, go error: %s", err))
		}
	}

	modelName := plan.ModelName.ValueString()
	createResp, err := r.client.Applications.CreateApplication(ctx,
		&juju.CreateApplicationInput{
			ApplicationName: plan.ApplicationName.ValueString(),
			ModelName:       modelName,
			CharmName:       charmName,
			CharmChannel:    channel,
			CharmRevision:   revision,
			CharmBase:       planCharm.Base.ValueString(),
			CharmSeries:     planCharm.Series.ValueString(),
			Units:           int(plan.UnitCount.ValueInt64()),
			Config:          configField,
			Constraints:     parsedConstraints,
			Trust:           plan.Trust.ValueBool(),
			Expose:          expose,
			Placement:       plan.Placement.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create application, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("create application resource %q", createResp.AppName))

	readResp, err := r.client.Applications.ReadApplicationWithRetryOnNotFound(ctx, &juju.ReadApplicationInput{
		ModelName: modelName,
		AppName:   createResp.AppName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read application, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("read application resource %q", createResp.AppName))

	// Save plan into Terraform state
	plan.Constraints = types.StringValue(readResp.Constraints.String())
	plan.Placement = types.StringValue(readResp.Placement)
	plan.Principal = types.BoolNull()
	plan.ApplicationName = types.StringValue(createResp.AppName)
	planCharm.Revision = types.Int64Value(int64(readResp.Revision))
	planCharm.Base = types.StringValue(readResp.Base)
	planCharm.Series = types.StringValue(readResp.Series)
	planCharm.Channel = types.StringValue(readResp.Channel)
	charmType := req.Config.Schema.GetBlocks()[CharmKey].(schema.ListNestedBlock).NestedObject.Type()
	var dErr diag.Diagnostics
	plan.Charm, dErr = types.ListValueFrom(ctx, charmType, []nestedCharm{planCharm})
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}
	plan.ID = types.StringValue(newAppID(plan.ModelName.ValueString(), createResp.AppName))
	r.trace("Created", applicationResourceModelForLogging(ctx, &plan))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func handleApplicationNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if errors.As(err, &juju.ApplicationNotFoundError) {
		// Application manually removed
		st.RemoveResource(ctx)
		return diag.Diagnostics{}
	}
	var diags diag.Diagnostics
	diags.AddError("Not Found", err.Error())
	return diags
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
// Take the juju api input from the ID, it may not exist in the plan.
// Only set optional values if they exist.
func (r *applicationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "application", "read")
		return
	}
	var state applicationResourceModel

	// Read Terraform prior state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Read", map[string]interface{}{
		"ID": state.ID.ValueString(),
	})

	modelName, appName, dErr := modelAppNameFromID(state.ID.ValueString())
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	response, err := r.client.Applications.ReadApplication(&juju.ReadApplicationInput{
		ModelName: modelName,
		AppName:   appName,
	})
	if err != nil {
		resp.Diagnostics.Append(handleApplicationNotFoundError(ctx, err, &resp.State)...)
		return
	}
	if response == nil {
		return
	}
	r.trace(fmt.Sprintf("read application resource %q", appName))

	state.ApplicationName = types.StringValue(appName)
	state.ModelName = types.StringValue(modelName)

	// Use the response to fill in state
	state.Placement = types.StringValue(response.Placement)
	state.Principal = types.BoolNull()
	state.UnitCount = types.Int64Value(int64(response.Units))
	state.Trust = types.BoolValue(response.Trust)

	// state requiring transformation
	dataCharm := nestedCharm{
		Name:     types.StringValue(response.Name),
		Channel:  types.StringValue(response.Channel),
		Revision: types.Int64Value(int64(response.Revision)),
		Base:     types.StringValue(response.Base),
		Series:   types.StringValue(response.Series),
	}
	charmType := req.State.Schema.GetBlocks()[CharmKey].(schema.ListNestedBlock).NestedObject.Type()
	state.Charm, dErr = types.ListValueFrom(ctx, charmType, []nestedCharm{dataCharm})
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	// constraints do not apply to subordinate applications.
	if response.Principal {
		state.Constraints = types.StringValue(response.Constraints.String())
	}
	exposeType := req.State.Schema.GetBlocks()[ExposeKey].(schema.ListNestedBlock).NestedObject.Type()
	if response.Expose != nil {
		exp := parseNestedExpose(response.Expose)
		state.Expose, dErr = types.ListValueFrom(ctx, exposeType, []nestedExpose{exp})
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	} else {
		state.Expose = types.ListNull(exposeType)
	}

	// we only set changes if there is any difference between
	// the previous and the current config values
	configType := req.State.Schema.GetAttributes()[ConfigKey].(schema.MapAttribute).ElementType
	state.Config, dErr = r.configureConfigData(ctx, configType, state.Config, response.Config)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	r.trace("Found", applicationResourceModelForLogging(ctx, &state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *applicationResource) configureConfigData(ctx context.Context, configType attr.Type, config types.Map, respCfg map[string]juju.ConfigEntry) (types.Map, diag.Diagnostics) {
	// We focus on those config entries that are not the default value.
	// If the value was the same we ignore it. If no changes were made,
	// jump to the next step.
	var previousConfig map[string]string
	diagErr := config.ElementsAs(ctx, &previousConfig, false)
	if diagErr.HasError() {
		r.trace("configureConfigData exit A")
		return types.Map{}, diagErr
	}
	if previousConfig == nil {
		previousConfig = make(map[string]string)
	}
	// known previously
	// update the values from the previous config
	changes := false
	for k, v := range respCfg {
		// Add if the value has changed from the previous state
		if previousValue, found := previousConfig[k]; found {
			if !juju.EqualConfigEntries(v, previousValue) {
				// remember that this terraform schema type only accepts strings
				previousConfig[k] = v.String()
				changes = true
			}
		} else if !v.IsDefault {
			// Add if the value is not default
			previousConfig[k] = v.String()
			changes = true
		}
	}
	if changes {
		return types.MapValueFrom(ctx, configType, previousConfig)
	}
	return config, nil
}

// Update is called to update the state of the resource. Config, planned
// state, and prior state values should be read from the
// UpdateRequest and new state values set on the UpdateResponse.
func (r *applicationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "application", "update")
		return
	}
	var plan, state applicationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Proposed update", applicationResourceModelForLogging(ctx, &plan))
	r.trace("Current state", applicationResourceModelForLogging(ctx, &state))

	updateApplicationInput := juju.UpdateApplicationInput{
		ModelName: state.ModelName.ValueString(),
		AppName:   state.ApplicationName.ValueString(),
	}

	if !plan.ApplicationName.IsUnknown() && !plan.ApplicationName.Equal(state.ApplicationName) {
		resp.Diagnostics.AddWarning("Unsupported", "unable to update application name")
	}

	if !plan.UnitCount.Equal(state.UnitCount) {
		updateApplicationInput.Units = intPtr(plan.UnitCount)
	}

	if !plan.Trust.Equal(state.Trust) {
		updateApplicationInput.Trust = plan.Trust.ValueBoolPointer()
	}

	if !plan.Charm.Equal(state.Charm) {
		var planCharms, stateCharms []nestedCharm
		resp.Diagnostics.Append(plan.Charm.ElementsAs(ctx, &planCharms, false)...)
		resp.Diagnostics.Append(state.Charm.ElementsAs(ctx, &stateCharms, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		planCharm := planCharms[0]
		stateCharm := stateCharms[0]
		if !planCharm.Channel.Equal(stateCharm.Channel) {
			updateApplicationInput.Channel = planCharm.Channel.ValueString()
		}
		if !planCharm.Series.Equal(stateCharm.Series) || !planCharm.Base.Equal(stateCharm.Base) {
			// This violates terraform's declarative model. We could implement
			// `juju set-application-base`, usually used after `upgrade-machine`,
			// which would change the operating system used for future units of
			// the application provided the charm supported it, but not change
			// the current. This provider does not implement an equivalent to
			// `upgrade-machine`. There is also a question of how to handle a
			// change to series, revision and channel at the same time.
			resp.Diagnostics.AddWarning("Not Supported", "Changing an application's operating system after deploy.")
		}
		if !planCharm.Revision.Equal(stateCharm.Revision) {
			updateApplicationInput.Revision = intPtr(planCharm.Revision)
		}
	}

	if !plan.Expose.Equal(state.Expose) {
		expose, unexpose, exposeDiags := r.computeExposeDeltas(ctx, state.Expose, plan.Expose)
		resp.Diagnostics.Append(exposeDiags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateApplicationInput.Expose = expose
		updateApplicationInput.Unexpose = unexpose
	}

	if !plan.Config.Equal(state.Config) {
		planConfigMap := map[string]string{}
		stateConfigMap := map[string]string{}
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &planConfigMap, false)...)
		resp.Diagnostics.Append(state.Config.ElementsAs(ctx, &stateConfigMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range planConfigMap {
			// we've lost the type of the config value. We compare the string
			// values.
			oldEntry := fmt.Sprintf("%#v", stateConfigMap[k])
			newEntry := fmt.Sprintf("%#v", v)
			if oldEntry != newEntry {
				if updateApplicationInput.Config == nil {
					// initialize just in case
					updateApplicationInput.Config = make(map[string]string)
				}
				updateApplicationInput.Config[k] = v
			}
		}
	}

	if !plan.Constraints.Equal(state.Constraints) {
		appConstraints, err := constraints.Parse(plan.Constraints.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Conversion", fmt.Sprintf("Unable to parse plan constraints, got error: %s", err))
		}
		updateApplicationInput.Constraints = &appConstraints
	}

	if err := r.client.Applications.UpdateApplication(&updateApplicationInput); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update application resource, got error: %s", err))
		return
	}

	plan.ID = types.StringValue(newAppID(plan.ModelName.ValueString(), plan.ApplicationName.ValueString()))
	plan.Principal = types.BoolNull()
	r.trace("Updated", applicationResourceModelForLogging(ctx, &plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// computeExposeDeltas computes the differences between the previously
// stored expose value and the current one. The valueSet argument is used
// to indicate whether the value was already set or not in the latest
// read of the plan.
func (r *applicationResource) computeExposeDeltas(ctx context.Context, stateExpose types.List, planExpose types.List) (map[string]interface{}, []string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	if planExpose.IsNull() {
		// if plan is nil we unexpose everything via
		// an non empty list.
		return nil, []string{""}, diags
	}
	if stateExpose.IsNull() {
		// State has no expose, but new plan does, setup for expose
		var planExposeSlice []nestedExpose
		diags.Append(planExpose.ElementsAs(ctx, &planExposeSlice, false)...)
		if diags.HasError() {
			return nil, []string{}, diags
		}
		if len(planExposeSlice) == 1 {
			return planExposeSlice[0].transformToMapStringInterface(), []string{}, diags
		}
		diags.AddError("Provider error", "plan expose has no objects, should be impossible")
		return nil, []string{}, diags
	}

	var planNestedExpose, stateNestedExpose []nestedExpose
	diags.Append(stateExpose.ElementsAs(ctx, &stateNestedExpose, false)...)
	if diags.HasError() {
		return nil, []string{}, diags
	}
	diags.Append(planExpose.ElementsAs(ctx, &planNestedExpose, false)...)
	if diags.HasError() {
		return nil, []string{}, diags
	}

	toExpose := make(map[string]interface{})
	toUnexpose := make([]string, 0)

	plan := planNestedExpose[0].transformToMapStringInterface()
	state := stateNestedExpose[0].transformToMapStringInterface()

	// if we have plan endpoints we have to expose them
	for endpoint, v := range plan {
		_, found := state[endpoint]
		if found {
			// this was already set
			// If it is different, unexpose and then expose
			if v != state[endpoint] {
				toUnexpose = append(toUnexpose, endpoint)
				toExpose[endpoint] = v
			}
		} else {
			// this was not set, expose it
			toExpose[endpoint] = v
		}
	}
	return toExpose, toUnexpose, diags
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
func (r *applicationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "application", "delete")
		return
	}
	var state applicationResourceModel
	// Read Terraform prior state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Deleting", map[string]interface{}{
		"ID": state.ID.ValueString(),
	})

	modelName, appName, dErr := modelAppNameFromID(state.ID.ValueString())
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
	}

	if err := r.client.Applications.DestroyApplication(&juju.DestroyApplicationInput{
		ApplicationName: appName,
		ModelName:       modelName,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete application, got error: %s", err))
	}
	r.trace(fmt.Sprintf("deleted application resource %q", state.ID.ValueString()))
}

// ImportState is called when the provider must import the state of a
// resource instance. This method must return enough state so the Read
// method can properly refresh the full resource.
//
// If setting an attribute with the import identifier, it is recommended
// to use the ImportStatePassthroughID() call in this method.
func (r *applicationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// ID is '<model name>:<app name>'
func newAppID(model, app string) string {
	return fmt.Sprintf("%s:%s", model, app)
}

func modelAppNameFromID(value string) (string, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	id := strings.Split(value, ":")
	//If importing with an incorrect ID we need to catch and provide a user-friendly error
	if len(id) != 2 {
		diags.AddError("Malformed ID", fmt.Sprintf("unable to parse model and application name from provided ID: %q", value))
		return "", "", diags
	}
	return id[0], id[1], diags
}

func (r *applicationResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(r.subCtx, LogResourceApplication, msg, additionalFields...)
}

func applicationResourceModelForLogging(_ context.Context, app *applicationResourceModel) map[string]interface{} {
	value := map[string]interface{}{
		"application-name": app.ApplicationName.ValueString(),
		"charm":            app.Charm.String(),
		"constraints":      app.Constraints.ValueString(),
		"model":            app.ModelName.ValueString(),
		"placement":        app.Placement.ValueString(),
		"expose":           app.Expose.String(),
		"trust":            app.Trust.ValueBoolPointer(),
		"units":            app.UnitCount.ValueInt64(),
	}
	return value
}
