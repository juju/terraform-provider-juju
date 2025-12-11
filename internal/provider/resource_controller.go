// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

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

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ resource.Resource = &controllerResource{}
var _ resource.ResourceWithConfigure = &controllerResource{}
var _ resource.ResourceWithImportState = &controllerResource{}

type controllerResourceModel struct {
	JujuBinary           types.String `tfsdk:"juju_binary"`
	Name                 types.String `tfsdk:"name"`
	Cloud                types.Object `tfsdk:"cloud"`
	CloudCredential      types.Object `tfsdk:"cloud_credential"`
	OutputFile           types.String `tfsdk:"output_file"`
	AgentVersion         types.String `tfsdk:"agent_version"`
	BootstrapBase        types.String `tfsdk:"bootstrap_base"`
	BootstrapConstraints types.Map    `tfsdk:"bootstrap_constraints"`
	Config               types.Map    `tfsdk:"config"`
	ModelDefault         types.Map    `tfsdk:"model_default"`
	ModelConstraints     types.Map    `tfsdk:"model_constraints"`
	StoragePool          types.Map    `tfsdk:"storage_pool"`

	APIAddresses types.List   `tfsdk:"api_addresses"`
	CACert       types.String `tfsdk:"ca_cert"`
	CAPrivateKey types.String `tfsdk:"ca_private_key"`
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// nestedCloudModel represents the cloud nested object in the controller resource
type nestedCloudModel struct {
	Name           types.String `tfsdk:"name"`
	AuthType       types.Set    `tfsdk:"auth_type"`
	CACertificates types.Set    `tfsdk:"ca_certificates"`
	Config         types.Map    `tfsdk:"config"`
	Endpoint       types.String `tfsdk:"endpoint"`
	Region         types.Object `tfsdk:"region"`
	Type           types.String `tfsdk:"type"`
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

// controllerConnectionInformation contains the connection details for a controller.
type controllerConnectionInformation struct {
	Addresses []string
	CACert    string
	Username  string
	Password  string
}

type boostrapArguments struct {
	AgentVersion         string
	BootstrapBase        string
	BootstrapConstraints map[string]string
	CAPrivateKey         string
	Cloud                bootstrapCloudArgument
	CloudCredential      bootstrapCredentialArgument
	Config               map[string]string
	JujuBinary           string
	ModelConstraints     map[string]string
	ModelDefault         map[string]string
	Name                 string
	OutputFile           string
	StoragePool          map[string]string
}

type bootstrapCloudArgument struct {
	Name           string
	AuthTypes      []string
	CACertificates []string
	Config         map[string]string
	Endpoint       string
	Region         *boostrapCloudRegionArgument
	Type           string
}

type boostrapCloudRegionArgument struct {
	Name             string
	Endpoint         string
	IdentityEndpoint string
	StorageEndpoint  string
}

type bootstrapCredentialArgument struct {
	Name       string
	AuthType   string
	Attributes map[string]string
}

type jujuCommand interface {
	// Bootstrap creates a new controller and returns connection information.
	Bootstrap(ctx context.Context, model boostrapArguments) (*controllerConnectionInformation, error)
	// UpdateConfig updates controller configuration.
	UpdateConfig(ctx context.Context, connInfo *controllerConnectionInformation, config map[string]string) error
	// Config retrieves controller configuration settings.
	Config(ctx context.Context, connInfo *controllerConnectionInformation) (map[string]string, error)
	// Destroy removes the controller.
	Destroy(ctx context.Context, connInfo *controllerConnectionInformation) error
}

var defaultNewJujuCommandFunction func(string) jujuCommand = func(jujuBinary string) jujuCommand {
	return &defaultJujuCommand{jujuBinary: jujuBinary}
}

// NewControllerResource returns a new resource for managing Juju controllers.
func NewControllerResource() resource.Resource {
	return &controllerResource{
		newJujuCommand: defaultNewJujuCommandFunction,
	}
}

type controllerResource struct {
	client         *juju.Client
	config         juju.Config
	newJujuCommand func(jujuBinary string) jujuCommand

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
				Sensitive:   false,
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
			},
			"ca_private_key": schema.StringAttribute{
				Description: "CA private key for the controller.",
				Sensitive:   true,
				Optional:    true,
			},
			"cloud": schema.SingleNestedAttribute{
				Description: "The cloud where the controller will operate.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
					objectplanmodifier.UseStateForUnknown(),
				},
				Required: true,
				Attributes: map[string]schema.Attribute{
					"auth_type": schema.SetAttribute{
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
					"name": schema.StringAttribute{
						Description: "The name of the cloud",
						Required:    true,
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
					mapplanmodifier.RequiresReplace(),
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name to be assigned to the controller. Changing this value will" +
					" require the controller to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"output_file": schema.StringAttribute{
				Description: "The name of the output file.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"password": schema.StringAttribute{
				Description: "Admin password for the controller.",
				Computed:    true,
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

	var cloudRegion *boostrapCloudRegionArgument
	if !cloudModel.Region.IsNull() && !cloudModel.Region.IsUnknown() {
		var regionModel nestedCloudRegionModel
		resp.Diagnostics.Append(cloudModel.Region.As(ctx, &regionModel, basetypes.ObjectAsOptions{})...)
		if resp.Diagnostics.HasError() {
			return
		}
		cloudRegion = &boostrapCloudRegionArgument{
			Name:             regionModel.Name.ValueString(),
			Endpoint:         regionModel.Endpoint.ValueString(),
			IdentityEndpoint: regionModel.IdentityEndpoint.ValueString(),
			StorageEndpoint:  regionModel.StorageEndpoint.ValueString(),
		}
	}

	var authTypes []string
	resp.Diagnostics.Append(cloudModel.AuthType.ElementsAs(ctx, &authTypes, false)...)
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

	bootstrapArgs := boostrapArguments{
		Name:                 plan.Name.ValueString(),
		JujuBinary:           plan.JujuBinary.ValueString(),
		OutputFile:           plan.OutputFile.ValueString(),
		AgentVersion:         plan.AgentVersion.ValueString(),
		BootstrapBase:        plan.BootstrapBase.ValueString(),
		BootstrapConstraints: bootstrapConstraints,
		CAPrivateKey:         plan.CAPrivateKey.ValueString(),
		Config:               config,
		ModelDefault:         modelDefault,
		ModelConstraints:     modelConstraints,
		StoragePool:          storagePool,
		Cloud: bootstrapCloudArgument{
			Name:           cloudModel.Name.ValueString(),
			AuthTypes:      authTypes,
			CACertificates: caCertificates,
			Config:         cloudConfig,
			Endpoint:       cloudModel.Endpoint.ValueString(),
			Region:         cloudRegion,
			Type:           cloudModel.Type.ValueString(),
		},
		CloudCredential: bootstrapCredentialArgument{
			Name:       credentialModel.Name.ValueString(),
			AuthType:   credentialModel.AuthType.ValueString(),
			Attributes: credentialAttributes,
		},
	}

	command := r.newJujuCommand(plan.JujuBinary.ValueString())
	result, err := command.Bootstrap(ctx, bootstrapArgs)
	if err != nil {
		resp.Diagnostics.AddError(
			"Bootstrap Error",
			fmt.Sprintf("Unable to bootstrap controller %q, got error: %s", plan.Name.ValueString(), err),
		)
		return
	}

	// Write output file with controller connection details
	outputFile := plan.OutputFile.ValueString()
	outputData := map[string]interface{}{
		"controller_name": plan.Name.ValueString(),
		"api_endpoints":   result.Addresses,
		"ca_cert":         result.CACert,
		"username":        result.Username,
		"password":        result.Password,
	}

	outputJSON, err := json.MarshalIndent(outputData, "", "  ")
	if err != nil {
		resp.Diagnostics.AddError(
			"JSON Marshal Error",
			fmt.Sprintf("Unable to marshal output data: %s", err),
		)
		return
	}

	if err := os.WriteFile(outputFile, outputJSON, 0600); err != nil {
		resp.Diagnostics.AddError(
			"Output File Write Error",
			fmt.Sprintf("Unable to write output file %q: %s", outputFile, err),
		)
		return
	}

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

	connInfo := &controllerConnectionInformation{
		Addresses: addresses,
		CACert:    state.CACert.ValueString(),
		Username:  state.Username.ValueString(),
		Password:  state.Password.ValueString(),
	}

	command := r.newJujuCommand(state.JujuBinary.ValueString())
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

	connInfo := &controllerConnectionInformation{
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

	command := r.newJujuCommand(state.JujuBinary.ValueString())
	err := command.UpdateConfig(ctx, connInfo, updatedConfig)
	if err != nil {
		resp.Diagnostics.AddError(
			"Controller Update Error",
			fmt.Sprintf("Unable to update controller %q configuration: %s", state.Name.ValueString(), err),
		)
		return
	}

	// Update the state with the new config
	state.Config, _ = types.MapValueFrom(ctx, types.StringType, updatedConfig)

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

	connInfo := &controllerConnectionInformation{
		Addresses: addresses,
		CACert:    state.CACert.ValueString(),
		Username:  state.Username.ValueString(),
		Password:  state.Password.ValueString(),
	}

	command := r.newJujuCommand(state.JujuBinary.ValueString())
	err := command.Destroy(ctx, connInfo)
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

// defaultJujuCommand implements jujuCommand using juju CLI
type defaultJujuCommand struct {
	jujuBinary string
}

func (d *defaultJujuCommand) Bootstrap(ctx context.Context, args boostrapArguments) (*controllerConnectionInformation, error) {
	// TODO: Implement bootstrap logic using d.jujuBinary
	return nil, fmt.Errorf("boostrap not implemented")
}

func (d *defaultJujuCommand) UpdateConfig(ctx context.Context, connInfo *controllerConnectionInformation, config map[string]string) error {
	// TODO: Implement config update logic
	return fmt.Errorf("update config not implemented")
}

func (d *defaultJujuCommand) Config(ctx context.Context, connInfo *controllerConnectionInformation) (map[string]string, error) {
	// TODO: Implement read logic
	return nil, fmt.Errorf("read not implemented")
}

func (d *defaultJujuCommand) Destroy(ctx context.Context, connInfo *controllerConnectionInformation) error {
	// TODO: Implement destroy logic
	return fmt.Errorf("not implemented")
}
