// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// JujuCommand defines the interface for interacting with Juju controllers.
type JujuCommand interface {
	// Bootstrap creates a new controller and returns connection information.
	Bootstrap(ctx context.Context, model juju.BootstrapArguments) (*juju.ControllerConnectionInformation, string, error)
	// UpdateConfig updates controller and controller-model configuration.
	UpdateConfig(ctx context.Context, connInfo *juju.ControllerConnectionInformation,
		controllerConfig, controllerModelConfig map[string]string,
		unsetControllerModelConfig []string) error
	// Config retrieves controller configuration and controller-model configuration settings.
	Config(ctx context.Context, connInfo *juju.ControllerConnectionInformation) (map[string]any, map[string]any, error)
	// Destroy removes the controller.
	Destroy(ctx context.Context, args juju.DestroyArguments) error
}

var _ resource.Resource = &controllerResource{}
var _ resource.ResourceWithConfigure = &controllerResource{}
var _ resource.ResourceWithImportState = &controllerResource{}

type controllerResourceModel struct {
	JujuBinary      types.String `tfsdk:"juju_binary"`
	Name            types.String `tfsdk:"name"`
	Cloud           types.Object `tfsdk:"cloud"`
	CloudCredential types.Object `tfsdk:"cloud_credential"`

	// Flags for bootstrap command
	AgentVersion         types.String `tfsdk:"agent_version"`
	BootstrapBase        types.String `tfsdk:"bootstrap_base"`
	BootstrapConstraints types.Map    `tfsdk:"bootstrap_constraints"`
	ModelDefault         types.Map    `tfsdk:"model_default"`
	ModelConstraints     types.Map    `tfsdk:"model_constraints"`
	StoragePool          types.Object `tfsdk:"storage_pool"`

	// Config that can be set at bootstrap
	BootstrapConfig       types.Map `tfsdk:"bootstrap_config"`
	ControllerConfig      types.Map `tfsdk:"controller_config"`
	ControllerModelConfig types.Map `tfsdk:"controller_model_config"`

	// Flags for destroy command
	DestroyFlags types.Object `tfsdk:"destroy_flags"`

	// Controller details
	APIAddresses types.List   `tfsdk:"api_addresses"`
	CACert       types.String `tfsdk:"ca_cert"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// nestedCloudModel represents the cloud nested object in the controller resource
type nestedCloudModel struct {
	Name            types.String `tfsdk:"name"`
	AuthTypes       types.Set    `tfsdk:"auth_types"`
	CACertificates  types.Set    `tfsdk:"ca_certificates"`
	Config          types.Map    `tfsdk:"config"`
	Endpoint        types.String `tfsdk:"endpoint"`
	HostCloudRegion types.String `tfsdk:"host_cloud_region"`
	Region          types.Object `tfsdk:"region"`
	Type            types.String `tfsdk:"type"`
}

// nestedCloudRegionModel represents the region nested object in the cloud
type nestedCloudRegionModel struct {
	Name             types.String `tfsdk:"name"`
	Endpoint         types.String `tfsdk:"endpoint"`
	IdentityEndpoint types.String `tfsdk:"identity_endpoint"`
	StorageEndpoint  types.String `tfsdk:"storage_endpoint"`
}

// nestedCloudCredentialModel represents cloud credentials configuration
type nestedCloudCredentialModel struct {
	Name       types.String `tfsdk:"name"`
	AuthType   types.String `tfsdk:"auth_type"`
	Attributes types.Map    `tfsdk:"attributes"`
}

// nestedStoragePoolModel represents storage pool configuration
type nestedStoragePoolModel struct {
	Name       types.String `tfsdk:"name"`
	Type       types.String `tfsdk:"type"`
	Attributes types.Map    `tfsdk:"attributes"`
}

// NewControllerResource returns a new resource for managing Juju controllers.
func NewControllerResource(newJujuCommand func(string) (JujuCommand, error)) resource.Resource {
	return &controllerResource{
		newJujuCommand: newJujuCommand,
	}
}

type controllerResource struct {
	client         *juju.Client
	config         juju.Config
	newJujuCommand func(jujuBinary string) (JujuCommand, error)

	// context for the logging subsystem.
	subCtx context.Context
}

// Schema defines the schema for the resource.
func (r *controllerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju Controller.",
		Attributes: map[string]schema.Attribute{
			"agent_version": schema.StringAttribute{
				Description: "The version of agent binaries.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"api_addresses": schema.ListAttribute{
				Description: "API addresses of the controller.",
				Computed:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"bootstrap_base": schema.StringAttribute{
				Description: "The base for the bootstrap machine.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"bootstrap_constraints": schema.MapAttribute{
				Description: "Constraints for the bootstrap machine.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"ca_cert": schema.StringAttribute{
				Description: "CA certificate for the controller.",
				Computed:    true,
				Sensitive:   false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"storage_pool": schema.SingleNestedAttribute{
				Description: "Options for the initial storage pool",
				Optional:    true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Description: "The name of the storage pool.",
						Required:    true,
					},
					"type": schema.StringAttribute{
						Description: "The storage pool type",
						Required:    true,
					},
					"attributes": schema.MapAttribute{
						Description: "Additional storage pool attributes.",
						Optional:    true,
						ElementType: types.StringType,
					},
				},
			},
			"cloud": schema.SingleNestedAttribute{
				Description: "The cloud where the controller will operate.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectplanmodifier.UseStateForUnknown(),
				},
				Required: true,
				Attributes: map[string]schema.Attribute{
					"auth_types": schema.SetAttribute{
						Description: "The authentication type(s) supported by the cloud.",
						Required:    true,
						ElementType: types.StringType,
					},
					"ca_certificates": schema.SetAttribute{
						Description: "CA certificates for the cloud.",
						Optional:    true,
						ElementType: types.StringType,
					},
					"config": schema.MapAttribute{
						Description: "Configuration options for the cloud.",
						Optional:    true,
						ElementType: types.StringType,
					},
					"endpoint": schema.StringAttribute{
						Description: "The API endpoint for the cloud.",
						Optional:    true,
					},
					"host_cloud_region": schema.StringAttribute{
						Description: "The host cloud region for the cloud.",
						Optional:    true,
					},
					"name": schema.StringAttribute{
						Description: "The name of the cloud",
						Required:    true,
						Validators: []validator.String{
							ValidatorMatchString(names.IsValidCloud, "invalid cloud name"),
						},
					},
					"region": schema.SingleNestedAttribute{
						Description: "The cloud region where the controller will operate.",
						Optional:    true,
						Attributes: map[string]schema.Attribute{
							"endpoint": schema.StringAttribute{
								Description: "The API endpoint for the region.",
								Required:    true,
							},
							"identity_endpoint": schema.StringAttribute{
								Description: "The identity endpoint for the region.",
								Optional:    true,
							},
							"name": schema.StringAttribute{
								Description: "The name of the region.",
								Required:    true,
							},
							"storage_endpoint": schema.StringAttribute{
								Description: "The storage endpoint for the region.",
								Optional:    true,
							},
						},
					},
					"type": schema.StringAttribute{
						Description: "The type of the cloud .",
						Required:    true,
					},
				},
			},
			"cloud_credential": schema.SingleNestedAttribute{
				Description: "Cloud credentials to use for bootstrapping the controller.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"attributes": schema.MapAttribute{
						Description: "Authentication attributes (key-value pairs specific to the auth type).",
						Required:    true,
						ElementType: types.StringType,
					},
					"auth_type": schema.StringAttribute{
						Description: "The authentication type (e.g., 'userpass', 'oauth2', 'access-key').",
						Required:    true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(1),
						},
					},
					"name": schema.StringAttribute{
						Description: "The name of the credential.",
						Required:    true,
					},
				},
			},

			// The configuration options below are applied during bootstrap.
			// Only certain values in controller_config and controller_model_config
			// can be changed after bootstrap. The use of a map[string]string
			// allows flexibility but will require normalisation when comparing
			// values between the user's plan and the controller's state.

			"bootstrap_config": schema.MapAttribute{
				Description: "Configuration options that apply during the bootstrap process.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
					mapplanmodifier.RequiresReplace(),
				},
			},
			"controller_model_config": schema.MapAttribute{
				Description: "Configuration options to be set for the controller model.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"controller_config": schema.MapAttribute{
				Description: "Configuration options for the bootstrapped controller. " +
					"Note that removing a key from this map will not unset it in the controller, " +
					"instead it will be left unchanged on the controller.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"juju_binary": schema.StringAttribute{
				Description: "The path to the juju CLI binary.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Default: stringdefault.StaticString("/usr/bin/juju"),
			},
			"model_constraints": schema.MapAttribute{
				Description: "Constraints for all workload machines in models.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"model_default": schema.MapAttribute{
				Description: "Configuration options to be set for all models.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name to be assigned to the controller. Changing this value will" +
					" require the controller to be destroyed and recreated by terraform.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"password": schema.StringAttribute{
				Description: "Admin password for the controller.",
				Computed:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Admin username for the controller.",
				Computed:    true,
				Sensitive:   false,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},

			// The flags below are only used when destroying the controller.
			"destroy_flags": schema.SingleNestedAttribute{
				Description: "Additional flags for destroying the controller.",
				Optional:    true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"destroy_all_models": schema.BoolAttribute{
						Description: "Destroy all models in the controller.",
						Optional:    true,
					},
					"destroy_storage": schema.BoolAttribute{
						Description: "Destroy all storage instances managed by the controller.",
						Optional:    true,
					},
					"force": schema.BoolAttribute{
						Description: "Force destroy models ignoring any errors.",
						Optional:    true,
					},
					"model_timeout": schema.Int32Attribute{
						Description: "Timeout for each step of force model destruction.",
						Optional:    true,
					},
					"release_storage": schema.BoolAttribute{
						Description: "Release all storage instances from management of the controller, without destroying them.",
						Optional:    true,
					},
				},
			},
		},
	}
}

// Configure prepares the resource for operations
func (r *controllerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceController)
}

// Metadata returns the resource type name.
func (r *controllerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_controller"
}

// ImportState imports the resource state.
func (r *controllerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// Create bootstraps a new Juju controller.
func (r *controllerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan controllerResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudModel nestedCloudModel
	resp.Diagnostics.Append(plan.Cloud.As(ctx, &cloudModel, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudRegion *juju.BootstrapCloudRegionArgument
	if !cloudModel.Region.IsNull() && !cloudModel.Region.IsUnknown() {
		var regionModel nestedCloudRegionModel
		resp.Diagnostics.Append(cloudModel.Region.As(ctx, &regionModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		cloudRegion = &juju.BootstrapCloudRegionArgument{
			Name:             regionModel.Name.ValueString(),
			Endpoint:         regionModel.Endpoint.ValueString(),
			IdentityEndpoint: regionModel.IdentityEndpoint.ValueString(),
			StorageEndpoint:  regionModel.StorageEndpoint.ValueString(),
		}
	}

	var authTypes []string
	resp.Diagnostics.Append(cloudModel.AuthTypes.ElementsAs(ctx, &authTypes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var caCertificates []string
	if !cloudModel.CACertificates.IsNull() && !cloudModel.CACertificates.IsUnknown() {
		resp.Diagnostics.Append(cloudModel.CACertificates.ElementsAs(ctx, &caCertificates, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var cloudConfig map[string]string
	if !cloudModel.Config.IsNull() && !cloudModel.Config.IsUnknown() {
		resp.Diagnostics.Append(cloudModel.Config.ElementsAs(ctx, &cloudConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var credentialModel nestedCloudCredentialModel
	resp.Diagnostics.Append(plan.CloudCredential.As(ctx, &credentialModel, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	var credentialAttributes map[string]string
	resp.Diagnostics.Append(credentialModel.Attributes.ElementsAs(ctx, &credentialAttributes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var bootstrapConstraints map[string]string
	if !plan.BootstrapConstraints.IsNull() && !plan.BootstrapConstraints.IsUnknown() {
		resp.Diagnostics.Append(plan.BootstrapConstraints.ElementsAs(ctx, &bootstrapConstraints, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var modelDefault map[string]string
	if !plan.ModelDefault.IsNull() && !plan.ModelDefault.IsUnknown() {
		resp.Diagnostics.Append(plan.ModelDefault.ElementsAs(ctx, &modelDefault, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var modelConstraints map[string]string
	if !plan.ModelConstraints.IsNull() && !plan.ModelConstraints.IsUnknown() {
		resp.Diagnostics.Append(plan.ModelConstraints.ElementsAs(ctx, &modelConstraints, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	storagePool := make(map[string]string)
	if !plan.StoragePool.IsNull() && !plan.StoragePool.IsUnknown() {
		var storagePoolModel nestedStoragePoolModel
		resp.Diagnostics.Append(plan.StoragePool.As(ctx, &storagePoolModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		storagePool["name"] = storagePoolModel.Name.ValueString()
		storagePool["type"] = storagePoolModel.Type.ValueString()
		var attributes map[string]string
		resp.Diagnostics.Append(storagePoolModel.Attributes.ElementsAs(ctx, &attributes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		maps.Copy(storagePool, attributes)
	}

	var controllerConfig map[string]string
	if !plan.ControllerConfig.IsNull() && !plan.ControllerConfig.IsUnknown() {
		resp.Diagnostics.Append(plan.ControllerConfig.ElementsAs(ctx, &controllerConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var bootstrapConfig map[string]string
	if !plan.BootstrapConfig.IsNull() && !plan.BootstrapConfig.IsUnknown() {
		resp.Diagnostics.Append(plan.BootstrapConfig.ElementsAs(ctx, &bootstrapConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var controllerModelConfig map[string]string
	if !plan.ControllerModelConfig.IsNull() && !plan.ControllerModelConfig.IsUnknown() {
		resp.Diagnostics.Append(plan.ControllerModelConfig.ElementsAs(ctx, &controllerModelConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	bootstrapArgs := juju.BootstrapArguments{
		Name:       plan.Name.ValueString(),
		JujuBinary: plan.JujuBinary.ValueString(),
		Cloud: juju.BootstrapCloudArgument{
			Name:            cloudModel.Name.ValueString(),
			AuthTypes:       authTypes,
			CACertificates:  caCertificates,
			Config:          cloudConfig,
			Endpoint:        cloudModel.Endpoint.ValueString(),
			HostCloudRegion: cloudModel.HostCloudRegion.ValueString(),
			Region:          cloudRegion,
			Type:            cloudModel.Type.ValueString(),
		},
		CloudCredential: juju.BootstrapCredentialArgument{
			Name:       credentialModel.Name.ValueString(),
			AuthType:   credentialModel.AuthType.ValueString(),
			Attributes: credentialAttributes,
		},
		Config: juju.BootstrapConfig{
			ControllerConfig:      controllerConfig,
			ControllerModelConfig: controllerModelConfig,
			BootstrapConfig:       bootstrapConfig,
		},
		Flags: juju.BootstrapFlags{
			AgentVersion:         plan.AgentVersion.ValueString(),
			BootstrapBase:        plan.BootstrapBase.ValueString(),
			BootstrapConstraints: buildStringListFromMap(bootstrapConstraints),
			ModelConstraints:     buildStringListFromMap(modelConstraints),
			ModelDefault:         buildStringListFromMap(modelDefault),
			StoragePool:          buildStringListFromMap(storagePool),
		},
	}

	command, err := r.newJujuCommand(plan.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", plan.JujuBinary.ValueString(), err),
		)
		return
	}
	result, version, err := command.Bootstrap(ctx, bootstrapArgs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Bootstrap Error",
			fmt.Sprintf("Unable to bootstrap controller %q, got error: %s", plan.Name.ValueString(), err),
		)
		return
	}

	r.trace(fmt.Sprintf("controller created: %q", plan.Name.ValueString()))

	plan.ID = types.StringValue(plan.Name.ValueString())

	// Store connection information in state
	apiAddresses, diags := types.ListValueFrom(ctx, types.StringType, result.Addresses)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.APIAddresses = apiAddresses
	plan.CACert = types.StringValue(result.CACert)
	plan.Username = types.StringValue(result.Username)
	plan.Password = types.StringValue(result.Password)
	plan.AgentVersion = types.StringValue(version)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read retrieves the Juju controller configuration.
func (r *controllerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state controllerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var addresses []string
	resp.Diagnostics.Append(state.APIAddresses.ElementsAs(ctx, &addresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connInfo := &juju.ControllerConnectionInformation{
		Addresses: addresses,
		CACert:    state.CACert.ValueString(),
		Username:  state.Username.ValueString(),
		Password:  state.Password.ValueString(),
	}

	command, err := r.newJujuCommand(state.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", state.JujuBinary.ValueString(), err),
		)
		return
	}
	controllerConfig, controllerModelConfig, err := command.Config(ctx, connInfo)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Read Error",
			fmt.Sprintf("Unable to read controller %q configuration: %s", state.Name.ValueString(), err),
		)
		return
	}

	cfg, diags := newConfigFromModelConfigAPI(ctx, controllerConfig, state.ControllerConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg == nil {
		state.ControllerConfig = types.MapNull(types.StringType)
	} else {
		state.ControllerConfig, diags = types.MapValueFrom(ctx, types.StringType, cfg)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	cfg, diags = newConfigFromModelConfigAPI(ctx, controllerModelConfig, state.ControllerModelConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if cfg == nil {
		state.ControllerModelConfig = types.MapNull(types.StringType)
	} else {
		state.ControllerModelConfig, diags = types.MapValueFrom(ctx, types.StringType, cfg)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the configuration of the Juju controller.
func (r *controllerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state controllerResourceModel
	var plan controllerResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var addresses []string
	resp.Diagnostics.Append(state.APIAddresses.ElementsAs(ctx, &addresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	connInfo := &juju.ControllerConnectionInformation{
		Addresses: addresses,
		CACert:    state.CACert.ValueString(),
		Username:  state.Username.ValueString(),
		Password:  state.Password.ValueString(),
	}

	// Note that below we ignore the unset controller config keys (besides warning on them)
	// because Juju's API does not support unsetting controller config values. If a user
	// removes a config key from their Terraform plan, it will be left unchanged in Juju.
	var diags diag.Diagnostics
	updatedControllerConfig, unsetControllerConfig, diags := computeConfigDiff(ctx, state.ControllerConfig, plan.ControllerConfig)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	for _, key := range unsetControllerConfig {
		resp.Diagnostics.AddWarning(
			"Controller Config Unset Warning",
			fmt.Sprintf("The controller config key %q was removed from the Terraform configuration, "+
				"but Juju does not support unsetting controller config values. The value will be left unchanged in the controller.",
				key),
		)
	}

	updatedControllerModelConfig, unsetControllerModelConfig, diags := computeConfigDiff(ctx, state.ControllerModelConfig, plan.ControllerModelConfig)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	command, err := r.newJujuCommand(state.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", plan.JujuBinary.ValueString(), err),
		)
		return
	}
	err = command.UpdateConfig(ctx, connInfo, updatedControllerConfig, updatedControllerModelConfig, unsetControllerModelConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Update Error",
			fmt.Sprintf("Unable to update controller %q configuration: %s", state.Name.ValueString(), err),
		)
		return
	}

	r.trace(fmt.Sprintf("controller updated: %q", plan.Name.ValueString()))

	// Write the updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete destroys the Juju controller.
func (r *controllerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state controllerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var addresses []string
	resp.Diagnostics.Append(state.APIAddresses.ElementsAs(ctx, &addresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	command, err := r.newJujuCommand(state.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", state.JujuBinary.ValueString(), err),
		)
		return
	}

	var cloudModel nestedCloudModel
	resp.Diagnostics.Append(state.Cloud.As(ctx, &cloudModel, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	var authTypes []string
	resp.Diagnostics.Append(cloudModel.AuthTypes.ElementsAs(ctx, &authTypes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var caCertificates []string
	if !cloudModel.CACertificates.IsNull() && !cloudModel.CACertificates.IsUnknown() {
		resp.Diagnostics.Append(cloudModel.CACertificates.ElementsAs(ctx, &caCertificates, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var cloudConfig map[string]string
	if !cloudModel.Config.IsNull() && !cloudModel.Config.IsUnknown() {
		resp.Diagnostics.Append(cloudModel.Config.ElementsAs(ctx, &cloudConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var credentialModel nestedCloudCredentialModel
	resp.Diagnostics.Append(state.CloudCredential.As(ctx, &credentialModel, basetypes.ObjectAsOptions{})...)
	if resp.Diagnostics.HasError() {
		return
	}

	var credentialAttributes map[string]string
	resp.Diagnostics.Append(credentialModel.Attributes.ElementsAs(ctx, &credentialAttributes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var cloudRegion *juju.BootstrapCloudRegionArgument
	if !cloudModel.Region.IsNull() && !cloudModel.Region.IsUnknown() {
		var regionModel nestedCloudRegionModel
		resp.Diagnostics.Append(cloudModel.Region.As(ctx, &regionModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		cloudRegion = &juju.BootstrapCloudRegionArgument{
			Name:             regionModel.Name.ValueString(),
			Endpoint:         regionModel.Endpoint.ValueString(),
			IdentityEndpoint: regionModel.IdentityEndpoint.ValueString(),
			StorageEndpoint:  regionModel.StorageEndpoint.ValueString(),
		}
	}

	var destroyFlags juju.DestroyFlags
	if !state.DestroyFlags.IsNull() && !state.DestroyFlags.IsUnknown() {
		resp.Diagnostics.Append(state.DestroyFlags.As(ctx, &destroyFlags, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	args := juju.DestroyArguments{
		Name:         state.Name.ValueString(),
		JujuBinary:   state.JujuBinary.ValueString(),
		AgentVersion: state.AgentVersion.ValueString(),
		Cloud: juju.BootstrapCloudArgument{
			Name:            cloudModel.Name.ValueString(),
			AuthTypes:       authTypes,
			CACertificates:  caCertificates,
			Config:          cloudConfig,
			Endpoint:        cloudModel.Endpoint.ValueString(),
			HostCloudRegion: cloudModel.HostCloudRegion.ValueString(),
			Region:          cloudRegion,
			Type:            cloudModel.Type.ValueString(),
		},
		CloudCredential: juju.BootstrapCredentialArgument{
			Name:       credentialModel.Name.ValueString(),
			AuthType:   credentialModel.AuthType.ValueString(),
			Attributes: credentialAttributes,
		},
		ConnectionInfo: juju.ControllerConnectionInformation{
			Addresses: addresses,
			CACert:    state.CACert.ValueString(),
			Username:  state.Username.ValueString(),
			Password:  state.Password.ValueString(),
		},
		Flags: destroyFlags,
	}

	err = command.Destroy(ctx, args)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Deletion Error",
			fmt.Sprintf("Unable to destroy controller %q, got error: %s", state.Name.ValueString(), err),
		)
		return
	}
}

// buildStringListFromMap converts a map to a list of key=value strings.
func buildStringListFromMap(constraints map[string]string) []string {
	if len(constraints) == 0 {
		return nil
	}
	parts := make([]string, 0, len(constraints))
	for k, v := range constraints {
		parts = append(parts, fmt.Sprintf("%s=%s", k, v))
	}
	return parts
}

func (r *controllerResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(r.subCtx, LogResourceController, msg, additionalFields...)
}
