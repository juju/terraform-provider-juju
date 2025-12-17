// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
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
	Bootstrap(ctx context.Context, model juju.BootstrapArguments) (*juju.ControllerConnectionInformation, error)
	// UpdateConfig updates controller configuration.
	UpdateConfig(ctx context.Context, connInfo *juju.ControllerConnectionInformation, config map[string]string) error
	// Config retrieves controller configuration settings.
	Config(ctx context.Context, connInfo *juju.ControllerConnectionInformation) (map[string]string, error)
	// Destroy removes the controller.
	Destroy(ctx context.Context, connInfo *juju.ControllerConnectionInformation) error
}

var _ resource.Resource = &controllerResource{}
var _ resource.ResourceWithConfigure = &controllerResource{}
var _ resource.ResourceWithImportState = &controllerResource{}

type controllerResourceModel struct {
	JujuBinary           types.String `tfsdk:"juju_binary"`
	Name                 types.String `tfsdk:"name"`
	Cloud                types.Object `tfsdk:"cloud"`
	CloudCredential      types.Object `tfsdk:"cloud_credential"`
	AgentVersion         types.String `tfsdk:"agent_version"`
	BootstrapBase        types.String `tfsdk:"bootstrap_base"`
	BootstrapConstraints types.Map    `tfsdk:"bootstrap_constraints"`
	BootstrapTimeout     types.String `tfsdk:"bootstrap_timeout"`
	Config               types.Map    `tfsdk:"config"`
	ModelDefault         types.Map    `tfsdk:"model_default"`
	ModelConstraints     types.Map    `tfsdk:"model_constraints"`
	StoragePool          types.Map    `tfsdk:"storage_pool"`

	APIAddresses              types.List   `tfsdk:"api_addresses"`
	AdminSecret               types.String `tfsdk:"admin_secret"`
	CACert                    types.String `tfsdk:"ca_cert"`
	CAPrivateKey              types.String `tfsdk:"ca_private_key"`
	ControllerExternalIPAddrs types.List   `tfsdk:"controller_external_ip_addresses"`
	ControllerExternalName    types.String `tfsdk:"controller_external_name"`
	ControllerServiceType     types.String `tfsdk:"controller_service_type"`
	SSHServerHostKey          types.String `tfsdk:"ssh_server_host_key"`
	Username                  types.String `tfsdk:"username"`
	Password                  types.String `tfsdk:"password"`

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
			"admin_secret": schema.StringAttribute{
				Description: "The admin secret for the controller.",
				Optional:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"api_addresses": schema.ListAttribute{
				Description: "API addresses of the controller.",
				Computed:    true,
				ElementType: types.StringType,
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
			"bootstrap_timeout": schema.StringAttribute{
				Description: "The timeout for the bootstrap operation.",
				Optional:    true,
			},
			"ca_cert": schema.StringAttribute{
				Description: "CA certificate for the controller.",
				Computed:    true,
				Sensitive:   false,
			},
			"ca_private_key": schema.StringAttribute{
				Description: "CA private key for the controller.",
				Sensitive:   true,
				Optional:    true,
			},
			"controller_external_ip_addresses": schema.ListAttribute{
				Description: "External IP addresses for the controller.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"controller_external_name": schema.StringAttribute{
				Description: "External name for the controller.",
				Optional:    true,
			},
			"controller_service_type": schema.StringAttribute{
				Description: "Kubernetes service type for Juju controller. Valid values are one of cluster, loadbalancer and external.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.OneOfCaseInsensitive("cluster", "loadbalancer", "external"),
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
			"config": schema.MapAttribute{
				Description: "Configuration options for the bootstrapped controller.",
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
			},
			"ssh_server_host_key": schema.StringAttribute{
				Description: "SSH server host key for the controller.",
				Optional:    true,
				Sensitive:   true,
			},
			"storage_pool": schema.MapAttribute{
				Description: "Options for the initial storage pool (name and type are required plus any additional attributes).",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Admin username for the controller.",
				Computed:    true,
				Sensitive:   false,
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

	var config map[string]string
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &config, false)...)
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

	var storagePool map[string]string
	if !plan.StoragePool.IsNull() && !plan.StoragePool.IsUnknown() {
		resp.Diagnostics.Append(plan.StoragePool.ElementsAs(ctx, &storagePool, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	var controllerExternalIPAddrs []string
	if !plan.ControllerExternalIPAddrs.IsNull() && !plan.ControllerExternalIPAddrs.IsUnknown() {
		resp.Diagnostics.Append(plan.ControllerExternalIPAddrs.ElementsAs(ctx, &controllerExternalIPAddrs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	bootstrapArgs := juju.BootstrapArguments{
		AdminSecret:               plan.AdminSecret.ValueString(),
		AgentVersion:              plan.AgentVersion.ValueString(),
		BootstrapBase:             plan.BootstrapBase.ValueString(),
		BootstrapConstraints:      bootstrapConstraints,
		BootstrapTimeout:          plan.BootstrapTimeout.ValueString(),
		CAPrivateKey:              plan.CAPrivateKey.ValueString(),
		Config:                    config,
		ControllerExternalIPAddrs: controllerExternalIPAddrs,
		ControllerExternalName:    plan.ControllerExternalName.ValueString(),
		ControllerServiceType:     plan.ControllerServiceType.ValueString(),
		JujuBinary:                plan.JujuBinary.ValueString(),
		ModelConstraints:          modelConstraints,
		ModelDefault:              modelDefault,
		Name:                      plan.Name.ValueString(),
		SSHServerHostKey:          plan.SSHServerHostKey.ValueString(),
		StoragePool:               storagePool,
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
	}

	command, err := r.newJujuCommand(plan.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", plan.JujuBinary.ValueString(), err),
		)
		return
	}
	result, err := command.Bootstrap(ctx, bootstrapArgs)
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
	controllerConfig, err := command.Config(ctx, connInfo)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Read Error",
			fmt.Sprintf("Unable to read controller %q configuration: %s", state.Name.ValueString(), err),
		)
		return
	}

	if len(controllerConfig) > 0 {
		configMap, diags := types.MapValueFrom(ctx, types.StringType, controllerConfig)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Config = configMap
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

	var currentConfig map[string]string
	if !state.Config.IsNull() && !state.Config.IsUnknown() {
		resp.Diagnostics.Append(state.Config.ElementsAs(ctx, &currentConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		currentConfig = make(map[string]string)
	}

	var newConfig map[string]string
	if !plan.Config.IsNull() && !plan.Config.IsUnknown() {
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &newConfig, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	} else {
		newConfig = make(map[string]string)
	}

	updatedConfig := currentConfig
	for k, v := range newConfig {
		updatedConfig[k] = v
	}

	command, err := r.newJujuCommand(state.JujuBinary.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Juju Command Initialization Error",
			fmt.Sprintf("Unable to initialize Juju command using binary path %q: %s", plan.JujuBinary.ValueString(), err),
		)
		return
	}
	err = command.UpdateConfig(ctx, connInfo, updatedConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Update Error",
			fmt.Sprintf("Unable to update controller %q configuration: %s", state.Name.ValueString(), err),
		)
		return
	}

	// Update the state with the new config
	config, diags := types.MapValueFrom(ctx, types.StringType, updatedConfig)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	state.Config = config

	r.trace(fmt.Sprintf("controller updated: %q", plan.Name.ValueString()))

	// Write the updated state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
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
	err = command.Destroy(ctx, connInfo)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Deletion Error",
			fmt.Sprintf("Unable to destroy controller %q, got error: %s", state.Name.ValueString(), err),
		)
		return
	}
}

func (r *controllerResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(r.subCtx, LogResourceController, msg, additionalFields...)
}
