// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/core/constraints"
	jujustorage "github.com/juju/juju/storage"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

const (
	CharmKey            = "charm"
	CidrsKey            = "cidrs"
	ConfigKey           = "config"
	EndpointBindingsKey = "endpoint_bindings"
	EndpointsKey        = "endpoints"
	ExposeKey           = "expose"
	MachinesKey         = "machines"
	ResourceKey         = "resources"
	SpacesKey           = "spaces"
	StorageKey          = "storage"
	UnitsKey            = "units"

	imageRegistriesMarkdownDescription = `
	OCI image registry credentials for OCI images specified in the charm resources. The map key is the registry URL.

	If the charm resource requires authentication, supply a username and password that will be passed to the Juju API and added to the Kubernetes cluster.

	The registry credentials will only be used if the URL of the registry is a partial match for the OCI image URL specified in the charm resources.
	An OCI image URL is considered a match for a registry URL if the URL without the OCI image tag matches the registry URL. For example, 
	a charm OCI resource specified as "registry.example.com:5000/path/image:tag" will match a registry entry with key "registry.example.com:5000/path" 
	but not "registry.example.com:5000" nor "registry.example.com".
`
	resourceKeyMarkdownDescription = `
Charm resources. Must evaluate to a string. A resource could be a resource revision number from CharmHub or a custom OCI image resource.
Specify a resource other than the default for a charm. Note that not all charms have resources.

Notes:
* A resource can be specified by a revision number or by URL to a OCI image repository. Resources of type 'file' can only be specified by revision number. Resources of type 'oci-image' can be specified by revision number or URL.
* A resource can be added or changed at any time. If the charm has resources and None is specified in the plan, Juju will use the resource defined in the charm's specified channel.
* If a charm is refreshed, by changing the charm revision or channel and if the resource is specified by a revision in the plan, Juju will use the resource defined in the plan.
* Resources specified by URL to an OCI image repository will never be refreshed (upgraded) by juju during a charm refresh unless explicitly changed in the plan.
`
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &applicationResource{}
var _ resource.ResourceWithConfigure = &applicationResource{}
var _ resource.ResourceWithImportState = &applicationResource{}
var _ resource.ResourceWithUpgradeState = &applicationResource{}

// NewApplicationResource returns a new instance of the application resource responsible
// for managing Juju applications, including their configuration, charm, constraints, and
// related attributes.
func NewApplicationResource() resource.Resource {
	return &applicationResource{}
}

type applicationResource struct {
	client         *juju.Client
	providerConfig juju.Config

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type registryDetails struct {
	User     types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

type applicationResourceModel struct {
	ApplicationName   types.String           `tfsdk:"name"`
	Charm             types.List             `tfsdk:"charm"`
	Config            types.Map              `tfsdk:"config"`
	Constraints       CustomConstraintsValue `tfsdk:"constraints"`
	EndpointBindings  types.Set              `tfsdk:"endpoint_bindings"`
	Expose            types.List             `tfsdk:"expose"`
	Machines          types.Set              `tfsdk:"machines"`
	ModelType         types.String           `tfsdk:"model_type"`
	Resources         types.Map              `tfsdk:"resources"`
	StorageDirectives types.Map              `tfsdk:"storage_directives"`
	Storage           types.Set              `tfsdk:"storage"`
	Trust             types.Bool             `tfsdk:"trust"`
	UnitCount         types.Int64            `tfsdk:"units"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// applicationResourceModelV0 describes the application data model.
// tfsdk must match user resource schema attribute names.
type applicationResourceModelV0 struct {
	applicationResourceModel
	ModelName types.String `tfsdk:"model"`
	Placement types.String `tfsdk:"placement"`
	Principal types.Bool   `tfsdk:"principal"`
}

// applicationResourceModelV1 describes the application data model.
// tfsdk must match user resource schema attribute names.
type applicationResourceModelV1 struct {
	applicationResourceModel
	RegistryCredentials map[string]registryDetails `tfsdk:"registry_credentials"`
	ModelUUID           types.String               `tfsdk:"model_uuid"`
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

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = provider.Client
	r.providerConfig = provider.Config
	// Create the local logging subsystem here, using the TF context when creating it.
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceApplication)
}

func (r *applicationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a single Juju application deployment from a charm. Deployment of bundles" +
			" is not supported.",
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "A custom name for the application deployment. If empty, uses the charm's name." +
					"Changing this value will cause the application to be destroyed and recreated by terraform.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			MachinesKey: schema.SetAttribute{
				ElementType: types.StringType,
				Description: "Specify the target machines for the application's units. The number of machines in the set indicates" +
					" the unit count for the application. Removing a machine from the set will remove the application's unit residing on it." +
					" `machines` is mutually exclusive with `units`.",
				Optional: true,
				Computed: true,
				Validators: []validator.Set{
					setvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(UnitsKey),
					}...),
				},
			},
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the application is to be deployed. Changing this value" +
					" will cause the application to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"model_type": schema.StringAttribute{
				Description: "The type of the model where the application is deployed. It is a computed field and " +
					"is needed to determine if the application should be replaced or updated in case of base updates.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			UnitsKey: schema.Int64Attribute{
				Description: "The number of application units to deploy for the charm.",
				Optional:    true,
				Computed:    true,
				//Default:     int64default.StaticInt64(int64(1)),
				PlanModifiers: []planmodifier.Int64{
					UnitCountModifier(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			ConfigKey: schema.MapAttribute{
				Description: "Application specific configuration. Must evaluate to a string, integer or boolean.",
				Optional:    true,
				ElementType: types.StringType,
			},
			ConstraintsKey: schema.StringAttribute{
				CustomType: CustomConstraintsType{},
				Description: "Constraints imposed on this application. Changing this value will cause the" +
					" application to be destroyed and recreated by terraform.",
				Optional: true,
				// Set as "computed" to pre-populate and preserve any implicit constraints
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(constraintsRequiresReplacefunc, "", ""),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"storage_directives": schema.MapAttribute{
				Description: "Storage directives (constraints) for the juju application." +
					" The map key is the label of the storage defined by the charm," +
					" the map value is the storage directive in the form <pool>,<count>,<size>." +
					" Changing an existing key/value pair will cause the application to be replaced." +
					" Adding a new key/value pair will add storage to the application on upgrade.",
				ElementType: types.StringType,
				Optional:    true,
				Validators: []validator.Map{
					stringIsStorageDirectiveValidator{},
				},
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplaceIf(storageDirectivesMapRequiresReplace, "", ""),
				},
			},
			"storage": schema.SetNestedAttribute{
				Description: "Storage used by the application.",
				Optional:    true,
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label": schema.StringAttribute{
							Description: "The specific storage option defined in the charm.",
							Computed:    true,
						},
						"size": schema.StringAttribute{
							Description: "The size of each volume.",
							Computed:    true,
						},
						"pool": schema.StringAttribute{
							Description: "Name of the storage pool.",
							Computed:    true,
						},
						"count": schema.Int64Attribute{
							Description: "The number of volumes.",
							Computed:    true,
						},
					},
				},
			},
			"trust": schema.BoolAttribute{
				Description: "Set the trust for the application.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			EndpointBindingsKey: schema.SetNestedAttribute{
				Description: "Configure endpoint bindings",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"endpoint": schema.StringAttribute{
							Description: "Name of the endpoint to bind to a space. Keep null (or undefined) to define default binding.",
							Optional:    true,
						},
						"space": schema.StringAttribute{
							Description: "Name of the space to bind the endpoint to.",
							Required:    true,
						},
					},
				},
				Validators: []validator.Set{
					setNestedIsAttributeUniqueValidator{
						PathExpressions: path.MatchRelative().AtAnySetValue().MergeExpressions(path.MatchRelative().AtName("endpoint")),
					},
				},
			},
			"registry_credentials": schema.MapNestedAttribute{
				// The key of this map is the registry URL.
				Optional:            true,
				MarkdownDescription: imageRegistriesMarkdownDescription,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"username": schema.StringAttribute{
							Description: "The username for authenticating to the registry.",
							Required:    true,
						},
						"password": schema.StringAttribute{
							Description: "The password for authenticating to the registry.",
							Required:    true,
							Sensitive:   true,
						},
					},
				},
			},
			ResourceKey: schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.Map{
					StringIsResourceKeyValidator{},
				},
				MarkdownDescription: resourceKeyMarkdownDescription,
			},
		},
		Blocks: map[string]schema.Block{
			CharmKey: schema.ListNestedBlock{
				Description: "The charm installed from Charmhub.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
							Description: "The name of the charm to be deployed.  Changing this value will cause" +
								" the application to be destroyed and recreated by terraform.",
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
							Validators: []validator.String{
								StringIsChannelValidator{},
							},
						},
						"revision": schema.Int64Attribute{
							Description: "The revision of the charm to deploy. During the update phase, the charm revision should be update before config update, to avoid issues with config parameters parsing.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.Int64{
								int64planmodifier.UseStateForUnknown(),
							},
						},
						BaseKey: schema.StringAttribute{
							Description: "The operating system on which to deploy. E.g. ubuntu@22.04. Changing this value for machine charms will trigger a replace by terraform.",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
								stringplanmodifier.RequiresReplaceIf(baseApplicationRequiresReplaceIf, "", ""),
							},
							Validators: []validator.String{
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

// nestedEndpointBinding represents the single element of endpoint_bindings
// ListNestedAttribute
type nestedEndpointBinding struct {
	Endpoint types.String `tfsdk:"endpoint"`
	Space    types.String `tfsdk:"space"`
}

func (n nestedEndpointBinding) transformToStringTuple() (string, string) {
	return n.Endpoint.ValueString(), n.Space.ValueString()
}

// nestedStorage represents the single element of the storage SetNestedAttribute
type nestedStorage struct {
	Label types.String `tfsdk:"label"`
	Size  types.String `tfsdk:"size"`
	Pool  types.String `tfsdk:"pool"`
	Count types.Int64  `tfsdk:"count"`
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

	var plan applicationResourceModelV1

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

	config, diags := newConfig(ctx, plan.Config)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resourceRevisions := make(map[string]string)
	resp.Diagnostics.Append(plan.Resources.ElementsAs(ctx, &resourceRevisions, false)...)
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

	// Parse endpoint bindings
	var endpointBindings map[string]string
	if !plan.EndpointBindings.IsNull() {
		var endpointBindingsSlice []nestedEndpointBinding
		resp.Diagnostics.Append(plan.EndpointBindings.ElementsAs(ctx, &endpointBindingsSlice, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		r.trace("Creating application, endpoint bindings values", map[string]interface{}{"endpointBindingsSlice": endpointBindingsSlice})
		if len(endpointBindingsSlice) > 0 {
			endpointBindings = make(map[string]string)
			for _, binding := range endpointBindingsSlice {
				key, value := binding.transformToStringTuple()
				endpointBindings[key] = value
			}
		}
	}

	// Parse storage
	var storageConstraints map[string]jujustorage.Constraints
	if !plan.StorageDirectives.IsUnknown() {
		storageDirectives := make(map[string]string)
		resp.Diagnostics.Append(plan.StorageDirectives.ElementsAs(ctx, &storageDirectives, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		storageConstraints = make(map[string]jujustorage.Constraints, len(storageDirectives))
		for k, v := range storageDirectives {
			result, err := jujustorage.ParseConstraints(v)
			if err != nil {
				resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse storage directives, got error: %s", err))
				return
			}
			storageConstraints[k] = result
		}
	}

	unitCount := int(plan.UnitCount.ValueInt64())
	machines := []string{}
	if !plan.Machines.IsUnknown() {
		resp.Diagnostics.Append(plan.Machines.ElementsAs(ctx, &machines, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		unitCount = len(machines)
	}

	charmResources, err := createCharmResources(resourceRevisions, plan.RegistryCredentials)
	if err != nil {
		resp.Diagnostics.AddError("Input Error", fmt.Sprintf("Unable to process charm resources, got error: %s", err))
		return
	}

	modelUUID := plan.ModelUUID.ValueString()
	createResp, err := r.client.Applications.CreateApplication(ctx,
		&juju.CreateApplicationInput{
			ApplicationName:    plan.ApplicationName.ValueString(),
			ModelUUID:          modelUUID,
			CharmName:          charmName,
			CharmChannel:       channel,
			CharmRevision:      revision,
			CharmBase:          planCharm.Base.ValueString(),
			Units:              unitCount,
			Config:             config,
			Constraints:        parsedConstraints,
			Trust:              plan.Trust.ValueBool(),
			Expose:             expose,
			Machines:           machines,
			EndpointBindings:   endpointBindings,
			Resources:          charmResources,
			StorageConstraints: storageConstraints,
		},
	)
	// If the application was partially created, record it to state
	// and return with an error so that TF marks it as tainted.
	var partialApp juju.ApplicationPartiallyCreatedError
	if errors.As(err, &partialApp) {
		plan.ID = types.StringValue(newAppID(plan.ModelUUID.ValueString(), partialApp.AppName))
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Application partially created then failed, got error: %s", err))
		return
	}
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create application, got error: %s", err))
		return
	}

	r.trace(fmt.Sprintf("create application resource %q", createResp.AppName))
	readResp, err := r.client.Applications.ReadApplicationWithRetryOnNotFound(ctx, &juju.ReadApplicationInput{
		ModelUUID: modelUUID,
		AppName:   createResp.AppName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read application, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("read application resource %q", createResp.AppName))
	// Save plan into Terraform state

	// Constraints do not apply to subordinate applications. If the application
	// is subordinate, the constraints will be set to the empty string.
	plan.Constraints = NewCustomConstraintsValue(readResp.Constraints.String())
	if readResp.Principal || readResp.Units > 0 {
		plan.UnitCount = types.Int64Value(int64(readResp.Units))
	} else {
		plan.UnitCount = types.Int64Value(1)
	}

	var dErr diag.Diagnostics
	plan.Machines, dErr = types.SetValueFrom(ctx, types.StringType, readResp.Machines)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	plan.ApplicationName = types.StringValue(createResp.AppName)
	plan.ModelType = types.StringValue(readResp.ModelType)
	planCharm.Revision = types.Int64Value(int64(readResp.Revision))
	planCharm.Base = types.StringValue(readResp.Base)
	planCharm.Channel = types.StringValue(readResp.Channel)
	charmType := req.Config.Schema.GetBlocks()[CharmKey].(schema.ListNestedBlock).NestedObject.Type()

	plan.Charm, dErr = types.ListValueFrom(ctx, charmType, []nestedCharm{planCharm})
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	storageType := req.Config.Schema.GetAttributes()[StorageKey].(schema.SetNestedAttribute).NestedObject.Type()
	nestedStorageSlice := make([]nestedStorage, 0, len(readResp.Storage))
	for name, storage := range readResp.Storage {
		humanizedSize := transformSizeToHumanizedFormat(storage.Size)
		nestedStorageSlice = append(nestedStorageSlice, nestedStorage{
			Label: types.StringValue(name),
			Size:  types.StringValue(humanizedSize),
			Pool:  types.StringValue(storage.Pool),
			Count: types.Int64Value(int64(storage.Count)),
		})
	}
	if len(nestedStorageSlice) > 0 {
		plan.Storage, dErr = types.SetValueFrom(ctx, storageType, nestedStorageSlice)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	} else {
		plan.Storage = types.SetNull(storageType)
	}

	plan.ID = types.StringValue(newAppID(plan.ModelUUID.ValueString(), createResp.AppName))
	r.trace("Created", applicationResourceModelForLogging(ctx, &plan))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// createCharmResources processes the resources map specified
// and combines it with information on image registries.
// Each resource can be either a revision number or an OCI image URL.
// If the resource is a revision number, it is used as the charmRevision.
// If the resource is an OCI image URL, it is used as the ociImageURL.
// If the OCI image URL's registry matches one in the imageRegistries map,
// the corresponding username and password are used for authentication.
func createCharmResources(planResources map[string]string, imageRegistries map[string]registryDetails) (juju.CharmResources, error) {
	jujuResources := make(juju.CharmResources, len(planResources))
	for name, resource := range planResources {
		var charmRevision, ociImageURL, registryUser, registryPassword string

		if resource == "" {
			return nil, fmt.Errorf("resource for %q is an empty string", name)
		}

		if _, err := strconv.Atoi(resource); err == nil {
			charmRevision = resource
		} else {
			// Registry path matching is done based on a partial match.
			// An image with URL "registry.example.com:5000/path/image:tag"
			// will match a registry entry with key "registry.example.com:5000/path"
			// but not "registry.example.com:5000" nor "registry.example.com".
			//
			// This allows for different credentials to be used for different paths
			// within the same registry host e.g. github.com/canonical vs github.com/juju.
			// This ASSUMES that multiple images from the same org/namespace can use the same credentials.
			//
			// If no match is found, no authentication is used.
			ociImageURL = resource
			imageIndex := strings.LastIndex(ociImageURL, "/")
			registryPath := ociImageURL[:imageIndex]
			if registryDetails, ok := imageRegistries[registryPath]; ok {
				if !registryDetails.User.IsNull() {
					registryUser = registryDetails.User.ValueString()
				}
				if !registryDetails.Password.IsNull() {
					registryPassword = registryDetails.Password.ValueString()
				}
			}
		}

		jujuResources[name] = juju.CharmResource{
			RevisionNumber:   charmRevision,
			OCIImageURL:      ociImageURL,
			RegistryUser:     registryUser,
			RegistryPassword: registryPassword,
		}
	}

	return jujuResources, nil
}

func transformSizeToHumanizedFormat(size uint64) string {
	// remove the decimal point and the trailing zero
	formattedSize := strings.ReplaceAll(humanize.Bytes(size*humanize.MByte), ".0", "")
	// remove all spaces
	formattedSize = strings.ReplaceAll(formattedSize, " ", "")
	// remove the B at the end
	formattedSize = formattedSize[:len(formattedSize)-1]
	return formattedSize
}

func handleApplicationNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if errors.Is(err, juju.ApplicationNotFoundError) {
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
	var state applicationResourceModelV1

	// Read Terraform prior state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Read", map[string]interface{}{
		"ID": state.ID.ValueString(),
	})

	modelUUID, appName, dErr := modelAppNameFromID(state.ID.ValueString())
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	response, err := r.client.Applications.ReadApplication(&juju.ReadApplicationInput{
		ModelUUID: modelUUID,
		AppName:   appName,
	})
	if err != nil {
		resp.Diagnostics.Append(handleApplicationNotFoundError(ctx, err, &resp.State)...)
		return
	}
	if response == nil {
		return
	}
	r.trace("read application", map[string]interface{}{"resource": appName, "response": response})

	modelType, err := r.client.Applications.ModelType(modelUUID)
	if err != nil {
		resp.Diagnostics.Append(handleApplicationNotFoundError(ctx, err, &resp.State)...)
		return
	}

	state.ApplicationName = types.StringValue(appName)
	state.ModelUUID = types.StringValue(modelUUID)

	// Use the response to fill in state

	if response.Principal || response.Units > 0 {
		state.UnitCount = types.Int64Value(int64(response.Units))
	} else {
		state.UnitCount = types.Int64Value(1)
	}

	state.ModelType = types.StringValue(modelType.String())
	state.Trust = types.BoolValue(response.Trust)

	if len(response.Machines) > 0 {
		state.Machines, dErr = types.SetValueFrom(ctx, types.StringType, response.Machines)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	}

	// state requiring transformation
	dataCharm := nestedCharm{
		Name:     types.StringValue(response.Name),
		Channel:  types.StringValue(response.Channel),
		Revision: types.Int64Value(int64(response.Revision)),
		Base:     types.StringValue(response.Base),
	}
	charmType := req.State.Schema.GetBlocks()[CharmKey].(schema.ListNestedBlock).NestedObject.Type()
	state.Charm, dErr = types.ListValueFrom(ctx, charmType, []nestedCharm{dataCharm})
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	// Constraints do not apply to subordinate applications. If the application
	// is subordinate, the constraints will be set to the empty string.
	state.Constraints = NewCustomConstraintsValue(response.Constraints.String())

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

	// Config
	if len(response.Config) > 0 {
		config, diags := newConfigFromApplicationAPI(ctx, response.Config, state.Config)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Config, diags = types.MapValueFrom(ctx, types.StringType, config)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	endpointBindingsType := req.State.Schema.GetAttributes()[EndpointBindingsKey].(schema.SetNestedAttribute).NestedObject.Type()
	if len(response.EndpointBindings) > 0 {
		state.EndpointBindings, dErr = r.toEndpointBindingsSet(ctx, endpointBindingsType, response.EndpointBindings)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	}

	// convert the storage map to a list of nestedStorage
	nestedStorageSlice := make([]nestedStorage, 0, len(response.Storage))
	for name, storage := range response.Storage {
		humanizedSize := transformSizeToHumanizedFormat(storage.Size)
		nestedStorageSlice = append(nestedStorageSlice, nestedStorage{
			Label: types.StringValue(name),
			Size:  types.StringValue(humanizedSize),
			Pool:  types.StringValue(storage.Pool),
			Count: types.Int64Value(int64(storage.Count)),
		})
	}
	storageType := req.State.Schema.GetAttributes()[StorageKey].(schema.SetNestedAttribute).NestedObject.Type()
	if len(nestedStorageSlice) > 0 {
		state.Storage, dErr = types.SetValueFrom(ctx, storageType, nestedStorageSlice)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	} else {
		state.Storage = types.SetNull(storageType)
	}

	resourceType := req.State.Schema.GetAttributes()[ResourceKey].(schema.MapAttribute).ElementType
	state.Resources, dErr = r.configureResourceData(ctx, resourceType, state.Resources, response.Resources)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	r.trace("Found", applicationResourceModelForLogging(ctx, &state))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Convert the endpoint bindings from the juju api to terraform nestedEndpointBinding set
func (r *applicationResource) toEndpointBindingsSet(ctx context.Context, endpointBindingsType attr.Type, endpointBindings map[string]string) (types.Set, diag.Diagnostics) {
	endpointBindingsSlice := make([]nestedEndpointBinding, 0, len(endpointBindings))
	for endpoint, space := range endpointBindings {
		var endpointString types.String
		if endpoint == "" {
			endpointString = types.StringNull()
		} else {
			endpointString = types.StringValue(endpoint)
		}
		endpointBindingsSlice = append(endpointBindingsSlice, nestedEndpointBinding{Endpoint: endpointString, Space: types.StringValue(space)})
	}

	return types.SetValueFrom(ctx, endpointBindingsType, endpointBindingsSlice)
}

func (r *applicationResource) configureResourceData(ctx context.Context, resourceType attr.Type, resources types.Map, respResources map[string]string) (types.Map, diag.Diagnostics) {
	var previousResources map[string]string
	diagErr := resources.ElementsAs(ctx, &previousResources, false)
	if diagErr.HasError() {
		r.trace("configureResourceData exit A")
		return types.Map{}, diagErr
	}
	if previousResources == nil {
		previousResources = make(map[string]string)
	}
	// known previously
	// update the values from the previous config
	changes := false
	for k, v := range respResources {
		// Add if the value has changed from the previous state
		if previousValue, found := previousResources[k]; found {
			if v != previousValue {
				// if the value is -1, it means this resource was uploaded, so we
				// get it from the state.
				if v != "-1" {
					previousResources[k] = v
				}
				changes = true
			}
		}
	}
	if changes {
		return types.MapValueFrom(ctx, resourceType, previousResources)
	}
	return resources, nil
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
	var plan, state applicationResourceModelV1
	// asserts are intended to be used after the application is update to
	// assert the update has achieved its intended effect.
	asserts := []wait.Assert[*juju.ReadApplicationResponse]{}

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Proposed update", applicationResourceModelForLogging(ctx, &plan))
	r.trace("Current state", applicationResourceModelForLogging(ctx, &state))

	updateApplicationInput := juju.UpdateApplicationInput{
		ModelUUID: state.ModelUUID.ValueString(),
		AppName:   state.ApplicationName.ValueString(),
	}

	if !plan.ApplicationName.IsUnknown() && !plan.ApplicationName.Equal(state.ApplicationName) {
		resp.Diagnostics.AddWarning("Unsupported", "unable to update application name")
	}

	if !plan.UnitCount.Equal(state.UnitCount) && (plan.Machines.IsNull() || plan.Machines.IsUnknown()) {
		updateApplicationInput.Units = intPtr(plan.UnitCount)

		// TODO (simonedutto): add assertion that the application has the
		// desired number of units
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
		} else {
			updateApplicationInput.Channel = stateCharm.Channel.ValueString()
		}

		if !planCharm.Revision.Equal(stateCharm.Revision) {
			updateApplicationInput.Revision = intPtr(planCharm.Revision)
		} else {
			updateApplicationInput.Revision = intPtr(stateCharm.Revision)
		}

		if !planCharm.Base.Equal(stateCharm.Base) {
			updateApplicationInput.Base = planCharm.Base.ValueString()
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
		newConfig, unsetKeys, diags := computeConfigDiff(ctx, state.Config, plan.Config)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateApplicationInput.Config = newConfig
		updateApplicationInput.UnsetConfig = unsetKeys
	}

	if !plan.Machines.Equal(state.Machines) {
		var planMachines, stateMachines []string
		if !(plan.Machines.IsUnknown() || plan.Machines.IsNull()) {
			resp.Diagnostics.Append(plan.Machines.ElementsAs(ctx, &planMachines, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
		if !(state.Machines.IsUnknown() || plan.Machines.IsUnknown()) {
			resp.Diagnostics.Append(state.Machines.ElementsAs(ctx, &stateMachines, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
		}

		addMachines := []string{}
		removeMachines := []string{}
		for _, planMachine := range planMachines {
			if !slices.Contains(stateMachines, planMachine) {
				addMachines = append(addMachines, planMachine)
			}
		}
		for _, stateMachine := range stateMachines {
			if !slices.Contains(planMachines, stateMachine) {
				removeMachines = append(removeMachines, stateMachine)
			}
		}
		updateApplicationInput.AddMachines = addMachines
		updateApplicationInput.RemoveMachines = removeMachines

		if len(planMachines) > 0 {
			asserts = append(asserts, assertEqualsMachines(planMachines))
		}
	}

	planResourceRevisions := make(map[string]string)
	resp.Diagnostics.Append(plan.Resources.ElementsAs(ctx, &planResourceRevisions, false)...)
	stateResourceRevisions := make(map[string]string)
	resp.Diagnostics.Append(state.Resources.ElementsAs(ctx, &stateResourceRevisions, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planResources, err := createCharmResources(planResourceRevisions, plan.RegistryCredentials)
	if err != nil {
		resp.Diagnostics.AddError("Input Error", fmt.Sprintf("Unable to process charm resources, got error: %s", err))
		return
	}
	stateResources, err := createCharmResources(stateResourceRevisions, state.RegistryCredentials)
	if err != nil {
		resp.Diagnostics.AddError("Input Error", fmt.Sprintf("Unable to process charm resources, got error: %s", err))
		return
	}

	// if resources in the plan are equal to resources stored in the state,
	// we pass on the resources specified in the plan, which tells the provider
	// NOT to update resources, because we want resources fixed to those
	// specified in the plan.
	if planResources.Equal(stateResources) {
		planResourceMap := make(map[string]string)
		resp.Diagnostics.Append(plan.Resources.ElementsAs(ctx, &planResourceMap, false)...)
		updateApplicationInput.Resources = planResources
	} else {
		// what happens when the plan suddenly does not specify resource
		// revisions, but state does.
		for k, v := range planResources {
			if stateResources[k] != v {
				if updateApplicationInput.Resources == nil {
					// initialize just in case
					updateApplicationInput.Resources = make(juju.CharmResources)
				}
				updateApplicationInput.Resources[k] = v
			}
		}
		// Check for any removed resources.
		// Then, the resources get updated to the latest resource revision according to channel
		for k := range stateResources {
			if _, found := planResources[k]; found {
				continue
			}
			if updateApplicationInput.Resources == nil {
				// initialize the resources
				updateApplicationInput.Resources = make(juju.CharmResources)
			}
			// Set resource revision to zero gets the latest resource revision from CharmHub
			updateApplicationInput.Resources[k] = juju.CharmResource{
				RevisionNumber: "-1",
			}
		}
	}

	// Do not use .Equal() here as we should consider null constraints the same
	// as empty-string constraints. Terraform considers them different, so will
	// incorrectly attempt to update the constraints, which can cause trouble
	// for subordinate applications.
	if plan.Constraints.ValueString() != state.Constraints.ValueString() {
		appConstraints, err := constraints.Parse(plan.Constraints.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Conversion", fmt.Sprintf("Unable to parse plan constraints, got error: %s", err))
		}
		updateApplicationInput.Constraints = &appConstraints
	}

	if !plan.EndpointBindings.Equal(state.EndpointBindings) {
		endpointBindings, dErr := r.computeEndpointBindingsDeltas(ctx, state.EndpointBindings, plan.EndpointBindings)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
		updateApplicationInput.EndpointBindings = endpointBindings
	}

	// Check if we have new storage in plan that not existed in the state, and add their constraints to the
	// update application input.
	if !plan.StorageDirectives.Equal(state.StorageDirectives) {
		directives, dErr := r.updateStorage(ctx, plan, state)
		resp.Diagnostics.Append(dErr...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateApplicationInput.StorageConstraints = directives
	}

	if err := r.client.Applications.UpdateApplication(&updateApplicationInput); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update application resource, got error: %s", err))
		return
	}

	readResp, err := wait.WaitFor(
		wait.WaitForCfg[*juju.ReadApplicationInput, *juju.ReadApplicationResponse]{
			Context: ctx,
			GetData: r.client.Applications.ReadApplication,
			Input: &juju.ReadApplicationInput{
				ModelUUID: updateApplicationInput.ModelUUID,
				AppName:   updateApplicationInput.AppName,
			},
			DataAssertions: asserts,
			NonFatalErrors: []error{juju.ConnectionRefusedError, juju.RetryReadError, juju.ApplicationNotFoundError, juju.StorageNotFoundError},
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read application resource after update, got error: %s", err))
		return
	}

	// If the plan has refreshed the charm, changed the unit count,
	// or changed placement, wait for the changes to be seen in
	// status. Including storage as it can be added on a refresh.
	storageType := req.Config.Schema.GetAttributes()[StorageKey].(schema.SetNestedAttribute).NestedObject.Type()

	var dErr diag.Diagnostics
	plan.Machines, dErr = types.SetValueFrom(ctx, types.StringType, readResp.Machines)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	if updateApplicationInput.Channel != "" ||
		updateApplicationInput.Revision != nil ||
		updateApplicationInput.Units != nil ||
		updateApplicationInput.Base != "" {
		var nestedStorageSlice []nestedStorage
		for name, storage := range readResp.Storage {
			humanizedSize := transformSizeToHumanizedFormat(storage.Size)
			nestedStorageSlice = append(nestedStorageSlice, nestedStorage{
				Label: types.StringValue(name),
				Size:  types.StringValue(humanizedSize),
				Pool:  types.StringValue(storage.Pool),
				Count: types.Int64Value(int64(storage.Count)),
			})
		}
		if len(nestedStorageSlice) > 0 {
			var dErr diag.Diagnostics
			plan.Storage, dErr = types.SetValueFrom(ctx, storageType, nestedStorageSlice)
			if dErr.HasError() {
				resp.Diagnostics.Append(dErr...)
				return
			}
		} else {
			plan.Storage = types.SetNull(storageType)
		}
	} else {
		if !state.Storage.IsUnknown() {
			plan.Storage = state.Storage
		} else {
			plan.Storage.IsNull()
		}
	}

	plan.ModelType = state.ModelType
	plan.ID = types.StringValue(newAppID(plan.ModelUUID.ValueString(), plan.ApplicationName.ValueString()))
	r.trace("Updated", applicationResourceModelForLogging(ctx, &plan))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// updateStorage compares the plan storage directives to the
// state storage directives, any new labels are returned to be
// added as storage constraints.
func (r *applicationResource) updateStorage(
	ctx context.Context,
	plan applicationResourceModelV1,
	state applicationResourceModelV1,
) (map[string]jujustorage.Constraints, diag.Diagnostics) {
	diagnostics := diag.Diagnostics{}
	var updatedStorageDirectivesMap map[string]jujustorage.Constraints

	var planStorageDirectives, stateStorageDirectives map[string]string
	diagnostics.Append(plan.StorageDirectives.ElementsAs(ctx, &planStorageDirectives, false)...)
	if diagnostics.HasError() {
		return updatedStorageDirectivesMap, diagnostics
	}
	diagnostics.Append(state.StorageDirectives.ElementsAs(ctx, &stateStorageDirectives, false)...)
	if diagnostics.HasError() {
		return updatedStorageDirectivesMap, diagnostics
	}

	// Create a map of updated storage directives that are in the plan but not in the state
	updatedStorageDirectivesMap = make(map[string]jujustorage.Constraints)
	for label, constraintString := range planStorageDirectives {
		if _, ok := stateStorageDirectives[label]; !ok {
			cons, err := jujustorage.ParseConstraints(constraintString)
			if err != nil {
				// Just in case, as this should have been validated out before now.
				diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse storage directives, got error: %s", err))
				continue
			}
			updatedStorageDirectivesMap[label] = cons
		}
	}

	return updatedStorageDirectivesMap, diagnostics
}

// computeExposeDeltas computes the differences between the previously
// stored expose value and the current one. The valueSet argument is used
// to indicate whether the value was already set or not in the latest
// read of the plan.
func (r *applicationResource) computeExposeDeltas(ctx context.Context, stateExpose types.List, planExpose types.List) (map[string]interface{}, []string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	if planExpose.IsNull() {
		// if plan is nil we unexpose everything via a non-empty list.
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

// computeEndpointBindingsDeltas computes the differences between the previously
// stored endpoint bindings value and the current one.
// It returns a map of endpoint bindings to bind and unbind.
// Unbinding is represented by an empty string, and means that the endpoint
// should bound to the default space.
func (*applicationResource) computeEndpointBindingsDeltas(ctx context.Context, stateEndpointBindings types.Set, planEndpointBindings types.Set) (map[string]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}

	var planEndpointBindingsSlice, stateEndpointBindingsSlice []nestedEndpointBinding
	diags.Append(planEndpointBindings.ElementsAs(ctx, &planEndpointBindingsSlice, false)...)
	diags.Append(stateEndpointBindings.ElementsAs(ctx, &stateEndpointBindingsSlice, false)...)
	if diags.HasError() {
		return map[string]string{}, diags
	}
	planEndpointBindingsMap := make(map[string]string)
	for _, binding := range planEndpointBindingsSlice {
		key, value := binding.transformToStringTuple()
		planEndpointBindingsMap[key] = value
	}

	for _, binding := range stateEndpointBindingsSlice {
		key, _ := binding.transformToStringTuple()
		if _, ok := planEndpointBindingsMap[key]; !ok {
			// this was unset in the plan, unbind it
			planEndpointBindingsMap[key] = ""
		}
	}

	return planEndpointBindingsMap, nil
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
	var state applicationResourceModelV1
	// Read Terraform prior state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.trace("Deleting", map[string]interface{}{
		"ID": state.ID.ValueString(),
	})

	modelUUID, appName, dErr := modelAppNameFromID(state.ID.ValueString())
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
	}

	if err := r.client.Applications.DestroyApplication(&juju.DestroyApplicationInput{
		ApplicationName: appName,
		ModelUUID:       modelUUID,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete application, got error: %s", err))
	}

	err := wait.WaitForError(
		wait.WaitForErrorCfg[*juju.ReadApplicationInput, *juju.ReadApplicationResponse]{
			Context: ctx,
			GetData: r.client.Applications.ReadApplication,
			Input: &juju.ReadApplicationInput{
				ModelUUID: modelUUID,
				AppName:   appName,
			},
			ExpectedErr:    juju.ApplicationNotFoundError,
			RetryAllErrors: true,
		},
	)
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete application %q deletion: %v\n", appName, err)
		if r.providerConfig.SkipFailedDeletion {
			resp.Diagnostics.AddWarning(
				errSummary,
				errDetail+"There might be dangling resources requiring manual intervion.\n",
			)
		} else {
			resp.Diagnostics.AddError(
				errSummary,
				errDetail,
			)
		}
		return
	}

	r.trace(fmt.Sprintf("deleted application resource %q", state.ID.ValueString()))
}

// UpgradeState upgrades the state of the application resource.
// This is used to handle changes in the resource schema between versions.
func (o *applicationResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &appV0Schema,
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				appV0 := applicationResourceModelV0{}
				resp.Diagnostics.Append(req.State.Get(ctx, &appV0)...)

				if resp.Diagnostics.HasError() {
					return
				}

				modelUUID, err := o.client.Models.ModelUUID(appV0.ModelName.ValueString(), "")
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model UUID for model %q, got error: %s", appV0.ModelName.ValueString(), err))
					return
				}

				newID := newAppID(modelUUID, appV0.ApplicationName.ValueString())
				// appV0.ID is embedded in the applicationResourceModel struct.
				appV0.ID = types.StringValue(newID)

				upgradedStateData := applicationResourceModelV1{
					ModelUUID:                types.StringValue(modelUUID),
					applicationResourceModel: appV0.applicationResourceModel,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
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

// ID is '<model UUID>:<app name>'
func newAppID(modelUUID, app string) string {
	return fmt.Sprintf("%s:%s", modelUUID, app)
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

func applicationResourceModelForLogging(_ context.Context, app *applicationResourceModelV1) map[string]interface{} {
	value := map[string]interface{}{
		"application-name": app.ApplicationName.ValueString(),
		"charm":            app.Charm.String(),
		"constraints":      app.Constraints.ValueString(),
		"model_uuid":       app.ModelUUID.ValueString(),
		"expose":           app.Expose.String(),
		"trust":            app.Trust.ValueBoolPointer(),
		"units":            app.UnitCount.ValueInt64(),
		"storage":          app.Storage.String(),
	}
	return value
}

func assertEqualsMachines(machinesToCompare []string) func(outputFromAPI *juju.ReadApplicationResponse) error {
	return func(outputFromAPI *juju.ReadApplicationResponse) error {
		machineFromAPI := outputFromAPI.Machines

		pms := make([]string, len(machinesToCompare))
		copy(pms, machinesToCompare)

		slices.Sort(machineFromAPI)
		slices.Sort(machinesToCompare)

		if !slices.Equal(machineFromAPI, machinesToCompare) {
			return juju.NewRetryReadError("plan machines differ from application machines")
		}

		return nil
	}
}

// Below we store old schema definitions for the application resource.
// These are used to upgrade the state of the resource when the schema version changes.
// Keeping the v0 schema verbatim is the simplest solution currently and permits
// the design to change to something like a schema factory in the future.

var appV0Schema = schema.Schema{
	Description: "A resource that represents a single Juju application deployment from a charm. Deployment of bundles" +
		" is not supported.",
	Version: 0,
	Attributes: map[string]schema.Attribute{
		"name": schema.StringAttribute{
			Optional: true,
			Computed: true,
		},
		MachinesKey: schema.SetAttribute{
			ElementType: types.StringType,
			Optional:    true,
			Computed:    true,
		},
		"model": schema.StringAttribute{
			Required: true,
		},
		"model_type": schema.StringAttribute{
			Computed: true,
		},
		UnitsKey: schema.Int64Attribute{
			Optional: true,
			Computed: true,
		},
		ConfigKey: schema.MapAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
		ConstraintsKey: schema.StringAttribute{
			CustomType: CustomConstraintsType{},
			Optional:   true,
			// Set as "computed" to pre-populate and preserve any implicit constraints
			Computed: true,
		},
		"storage_directives": schema.MapAttribute{
			ElementType: types.StringType,
			Optional:    true,
		},
		"storage": schema.SetNestedAttribute{
			Optional: true,
			Computed: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"label": schema.StringAttribute{
						Computed: true,
					},
					"size": schema.StringAttribute{
						Computed: true,
					},
					"pool": schema.StringAttribute{
						Computed: true,
					},
					"count": schema.Int64Attribute{
						Computed: true,
					},
				},
			},
		},
		"trust": schema.BoolAttribute{
			Optional: true,
			Computed: true,
			Default:  booldefault.StaticBool(false),
		},
		"placement": schema.StringAttribute{
			Optional: true,
			Computed: true,
		},
		"principal": schema.BoolAttribute{
			Computed: true,
		},
		"id": schema.StringAttribute{
			Computed: true,
		},
		EndpointBindingsKey: schema.SetNestedAttribute{
			Optional: true,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"endpoint": schema.StringAttribute{
						Optional: true,
					},
					"space": schema.StringAttribute{
						Required: true,
					},
				},
			},
		},
		ResourceKey: schema.MapAttribute{
			Optional:    true,
			ElementType: types.StringType,
		},
	},
	Blocks: map[string]schema.Block{
		CharmKey: schema.ListNestedBlock{
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required: true,
					},
					"channel": schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
					"revision": schema.Int64Attribute{
						Optional: true,
						Computed: true,
					},
					BaseKey: schema.StringAttribute{
						Optional: true,
						Computed: true,
					},
				},
			},
		},
		ExposeKey: schema.ListNestedBlock{
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					EndpointsKey: schema.StringAttribute{
						Optional: true,
					},
					SpacesKey: schema.StringAttribute{
						Optional: true,
					},
					CidrsKey: schema.StringAttribute{
						Optional: true,
					},
				},
			},
		},
	},
}
