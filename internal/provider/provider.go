// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

const (
	JujuControllerEnvKey     = "JUJU_CONTROLLER_ADDRESSES"
	JujuUsernameEnvKey       = "JUJU_USERNAME"
	JujuPasswordEnvKey       = "JUJU_PASSWORD"
	JujuCACertEnvKey         = "JUJU_CA_CERT"
	JujuClientIDEnvKey       = "JUJU_CLIENT_ID"
	JujuClientSecretEnvKey   = "JUJU_CLIENT_SECRET"
	SkipFailedDeletionEnvKey = "JUJU_SKIP_FAILED_DELETION"

	ControllerMode          = "controller_mode"
	JujuController          = "controller_addresses"
	JujuUsername            = "username"
	JujuPassword            = "password"
	JujuClientID            = "client_id"
	JujuClientSecret        = "client_secret"
	JujuCACert              = "ca_certificate"
	SkipFailedDeletion      = "skip_failed_deletion"
	JujuOfferingControllers = "offering_controllers"

	TwoSourcesAuthWarning = "Two sources of identity for controller login"
)

// jujuProviderModelEnvVar gets the controller config,
// from environment variables.
func jujuProviderModelEnvVar(diags diag.Diagnostics) jujuProviderModel {
	// If parsing fails, issue a warning and default to false.
	skipFailedDeletionStrVal := os.Getenv(SkipFailedDeletionEnvKey)
	skipFailedDeletion, err := strconv.ParseBool(skipFailedDeletionStrVal)
	if err != nil {
		diags.AddWarning(fmt.Sprintf("Invalid value for %s", SkipFailedDeletion),
			fmt.Sprintf("The value %q is not a valid boolean. Defaulting to false.", skipFailedDeletionStrVal))
	}

	return jujuProviderModel{
		ControllerAddrs:    getEnvVar(JujuControllerEnvKey),
		CACert:             getEnvVar(JujuCACertEnvKey),
		ClientID:           getEnvVar(JujuClientIDEnvKey),
		ClientSecret:       getEnvVar(JujuClientSecretEnvKey),
		UserName:           getEnvVar(JujuUsernameEnvKey),
		Password:           getEnvVar(JujuPasswordEnvKey),
		SkipFailedDeletion: types.BoolValue(skipFailedDeletion),
	}
}

func jujuProviderModelLiveDiscovery() (jujuProviderModel, bool) {
	data := jujuProviderModel{}
	controllerConfig, cliNotExist := juju.GetLocalControllerConfig()
	if cliNotExist {
		return data, false
	}

	if ctrlAddrs, ok := controllerConfig[JujuControllerEnvKey]; ok && ctrlAddrs != "" {
		data.ControllerAddrs = types.StringValue(ctrlAddrs)
	}
	if caCert, ok := controllerConfig[JujuCACertEnvKey]; ok && caCert != "" {
		data.CACert = types.StringValue(caCert)
	}
	if user, ok := controllerConfig[JujuUsernameEnvKey]; ok && user != "" {
		data.UserName = types.StringValue(user)
	}
	if password, ok := controllerConfig[JujuPasswordEnvKey]; ok && password != "" {
		data.Password = types.StringValue(password)
	}
	return data, true
}

func getEnvVar(field string) types.String {
	value := types.StringNull()
	envVar := os.Getenv(field)
	if envVar != "" {
		// fall back to the live juju data
		value = types.StringValue(envVar)
	}
	return value
}

// Ensure jujuProvider satisfies various provider interfaces.
var _ provider.Provider = &jujuProvider{}

type ProviderConfiguration struct {
	WaitForResources bool
	NewJujuCommand   func(string) (JujuCommand, error)
}

// NewJujuProvider returns a framework style terraform provider.
func NewJujuProvider(version string, config ProviderConfiguration) provider.Provider {
	return &jujuProvider{
		version:          version,
		waitForResources: config.WaitForResources,
		newJujuCommand:   config.NewJujuCommand,
	}
}

type jujuProvider struct {
	version string

	// waitForResources is used to determine if the provider should wait for
	// resources to be created/destroyed before proceeding.
	waitForResources bool

	// newJujuCommand returns the implementation of the JujuCommand interface based on the provided Juju binary
	// to be used for controller management.
	newJujuCommand func(string) (JujuCommand, error)
}

type offeringControllerModel struct {
	ControllerAddrs types.String `tfsdk:"controller_addresses"`
	UserName        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	CACert          types.String `tfsdk:"ca_certificate"`
	ClientID        types.String `tfsdk:"client_id"`
	ClientSecret    types.String `tfsdk:"client_secret"`
}

type jujuProviderModel struct {
	ControllerMode  types.Bool   `tfsdk:"controller_mode"`
	ControllerAddrs types.String `tfsdk:"controller_addresses"`
	UserName        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	CACert          types.String `tfsdk:"ca_certificate"`
	ClientID        types.String `tfsdk:"client_id"`
	ClientSecret    types.String `tfsdk:"client_secret"`

	SkipFailedDeletion types.Bool `tfsdk:"skip_failed_deletion"`

	OfferingControllers types.Map `tfsdk:"offering_controllers"`
}

func (j jujuProviderModel) loginViaUsername() bool {
	return j.UserName.ValueString() != "" && j.Password.ValueString() != ""
}

func (j jujuProviderModel) loginViaClientCredentials() bool {
	return j.ClientID.ValueString() != "" && j.ClientSecret.ValueString() != ""
}

func (j jujuProviderModel) valid() bool {
	validUserPass := j.loginViaUsername()
	validClientCredentials := j.loginViaClientCredentials()

	return j.ControllerAddrs.ValueString() != "" &&
		(validUserPass || validClientCredentials) &&
		(!validUserPass || !validClientCredentials)
}

// merge 2 providerModels together. The receiver data takes
// precedence. If the model is valid after the client ID and
// client secret are set, return. They take precedence over
// username and password. The two combinations are also
// mutually exclusive. Diagnostic warning are returned if
// the new data contains a username but the current data has
// client ID.
func (j jujuProviderModel) merge(in jujuProviderModel, from string) (jujuProviderModel, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	mergedModel := j
	if mergedModel.SkipFailedDeletion.IsNull() {
		mergedModel.SkipFailedDeletion = in.SkipFailedDeletion
	}
	if mergedModel.ControllerAddrs.ValueString() == "" {
		mergedModel.ControllerAddrs = in.ControllerAddrs
	}
	if mergedModel.CACert.ValueString() == "" {
		mergedModel.CACert = in.CACert
	}
	if mergedModel.ClientID.ValueString() == "" {
		mergedModel.ClientID = in.ClientID
	}
	if mergedModel.ClientSecret.ValueString() == "" {
		mergedModel.ClientSecret = in.ClientSecret
	}
	if mergedModel.valid() {
		if in.UserName.ValueString() != "" {
			diags.AddWarning(TwoSourcesAuthWarning,
				fmt.Sprintf("Ignoring Username value. Username provided via %s,"+
					"however Client ID already available. Only one login type is possible.", from))
		}

		return mergedModel, diags
	}
	if mergedModel.UserName.ValueString() == "" {
		mergedModel.UserName = in.UserName
	}
	if mergedModel.Password.ValueString() == "" {
		mergedModel.Password = in.Password
	}
	return mergedModel, diags
}

// Metadata returns the metadata for the provider, such as
// a type name and version data.
func (p *jujuProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "juju"
	resp.Version = p.version
}

// Schema returns the schema for this provider, specifically
// it defines the juju controller config necessary to create
// a juju client.
func (p *jujuProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			ControllerMode: schema.BoolAttribute{
				Description: "If set to true, the provider will only allow managing `juju_controller` resources.",
				Optional:    true,
				Validators: []validator.Bool{
					boolvalidator.ConflictsWith(
						path.Expressions{path.MatchRoot(JujuController)}...,
					),
				},
			},
			JujuController: schema.StringAttribute{
				Description: fmt.Sprintf("This is the controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
				Optional:    true,
			},
			JujuUsername: schema.StringAttribute{
				Description: fmt.Sprintf("This is the username registered with the controller to be used. This can also be set by the `%s` environment variable", JujuUsernameEnvKey),
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(JujuClientID),
						path.MatchRoot(JujuClientSecret),
					}...),
				},
			},
			JujuPassword: schema.StringAttribute{
				Description: fmt.Sprintf("This is the password of the username to be used. This can also be set by the `%s` environment variable", JujuPasswordEnvKey),
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(JujuClientID),
						path.MatchRoot(JujuClientSecret),
					}...),
				},
			},
			JujuClientID: schema.StringAttribute{
				Description: fmt.Sprintf("If using JAAS: This is the client ID (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `%s` environment variable", JujuClientIDEnvKey),
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(JujuUsername),
						path.MatchRoot(JujuPassword),
					}...),
				},
			},
			JujuClientSecret: schema.StringAttribute{
				Description: fmt.Sprintf("If using JAAS: This is the client secret (OAuth2.0, created by the external identity provider) to be used. This can also be set by the `%s` environment variable", JujuClientSecretEnvKey),
				Optional:    true,
				Sensitive:   true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(JujuUsername),
						path.MatchRoot(JujuPassword),
					}...),
				},
			},
			JujuCACert: schema.StringAttribute{
				Description: fmt.Sprintf("If the controller was deployed with a self-signed certificate: This is the certificate to use for identification. This can also be set by the `%s` environment variable", JujuCACertEnvKey),
				Optional:    true,
			},
			SkipFailedDeletion: schema.BoolAttribute{
				Description: fmt.Sprintf("Whether to issue a warning instead of an error and continue if a resource deletion fails. This can also be set by the `%s` environment variable. Defaults to false.", SkipFailedDeletionEnvKey),
				Optional:    true,
			},
			JujuOfferingControllers: schema.MapNestedAttribute{
				Description: "Additional controller details for cross-model integrations. The map key is the controller name.",
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						JujuController: schema.StringAttribute{
							Description: "Controller addresses to connect to. Multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,....",
							Required:    true,
						},
						JujuUsername: schema.StringAttribute{
							Description: "Username registered with the controller.",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.AlsoRequires(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuPassword),
								}...),
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuClientID),
									path.MatchRelative().AtParent().AtName(JujuClientSecret),
								}...),
							},
						},
						JujuPassword: schema.StringAttribute{
							Description: "Password for the controller username.",
							Sensitive:   true,
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.AlsoRequires(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuUsername),
								}...),
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuClientID),
									path.MatchRelative().AtParent().AtName(JujuClientSecret),
								}...),
							},
						},
						JujuCACert: schema.StringAttribute{
							Description: "CA certificate for the controller if using a self-signed certificate.",
							Optional:    true,
						},
						JujuClientID: schema.StringAttribute{
							Description: "The client ID (OAuth2.0, created by the external identity provider) to be used.",
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.AlsoRequires(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuClientSecret),
								}...),
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuUsername),
									path.MatchRelative().AtParent().AtName(JujuPassword),
								}...),
							},
						},
						JujuClientSecret: schema.StringAttribute{
							Description: "The client secret (OAuth2.0, created by the external identity provider) to be used.",
							Sensitive:   true,
							Optional:    true,
							Validators: []validator.String{
								stringvalidator.AlsoRequires(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuClientID),
								}...),
								stringvalidator.ConflictsWith(path.Expressions{
									path.MatchRelative().AtParent().AtName(JujuUsername),
									path.MatchRelative().AtParent().AtName(JujuPassword),
								}...),
							},
						},
					},
				},
			},
		},
	}
}

// Configure is called at the beginning of the provider lifecycle, when
// Terraform sends to the provider the values the user specified in the
// provider configuration block. These are supplied in the
// ConfigureProviderRequest argument.
// Values from provider configuration are often used to initialise an
// API client, which should be stored on the struct implementing the
// Provider interface.
func (p *jujuProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data jujuProviderModel
	var diags diag.Diagnostics
	// Read Terraform configuration data into the juju provider model.
	diags.Append(req.Config.Get(ctx, &data)...)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if data.ControllerMode.ValueBool() {
		providerData := juju.ProviderData{
			Config: juju.Config{
				ControllerMode: true,
			},
		}
		resp.ResourceData = providerData
		resp.DataSourceData = providerData
		return
	}
	// Get data required for configuring the juju client.
	data, diags = getJujuProviderModel(ctx, data)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	controllerConfig := juju.ControllerConfiguration{
		ControllerAddresses: strings.Split(data.ControllerAddrs.ValueString(), ","),
		Username:            data.UserName.ValueString(),
		Password:            data.Password.ValueString(),
		CACert:              data.CACert.ValueString(),
		ClientID:            data.ClientID.ValueString(),
		ClientSecret:        data.ClientSecret.ValueString(),
	}
	client, err := juju.NewClient(ctx, controllerConfig, p.waitForResources)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create juju client, got error: %s", err))
		return
	}
	config := juju.Config{
		SkipFailedDeletion: data.SkipFailedDeletion.ValueBool(),
	}

	providerData := juju.ProviderData{
		Client: client,
		Config: config,
	}

	// Here we are testing that we can connect successfully to the Juju server
	// this prevents having logic to check the connection is OK in every function
	testConn, err := providerData.Client.Models.GetConnection(nil)
	if err != nil {
		resp.Diagnostics.Append(checkClientErr(err, controllerConfig)...)
		return
	}
	_ = testConn.Close()

	// Register additional offering controllers if configured
	if !data.OfferingControllers.IsNull() && !data.OfferingControllers.IsUnknown() {
		var offeringControllers map[string]offeringControllerModel
		diags := data.OfferingControllers.ElementsAs(ctx, &offeringControllers, false)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		for controllerName, controller := range offeringControllers {
			tflog.Info(ctx, "Registering offering offering controller", map[string]interface{}{
				"controller_name": controllerName,
			})

			err := providerData.Client.Offers.AddOfferingController(
				controllerName,
				juju.ControllerConfiguration{
					ControllerAddresses: strings.Split(controller.ControllerAddrs.ValueString(), ","),
					Username:            controller.UserName.ValueString(),
					Password:            controller.Password.ValueString(),
					CACert:              controller.CACert.ValueString(),
					ClientID:            controller.ClientID.ValueString(),
					ClientSecret:        controller.ClientSecret.ValueString(),
				},
			)
			if err != nil {
				resp.Diagnostics.AddError(
					"Error Registering Additional Offering Controller",
					fmt.Sprintf("An error was encountered while registering additional offering controller '%s': %s",
						controllerName, err.Error()),
				)
				return
			}

			tflog.Info(ctx, "Successfully registered additional offering controller", map[string]interface{}{
				"controller_name": controllerName,
			})
		}
	}

	resp.ResourceData = providerData
	resp.DataSourceData = providerData
}

// getJujuProviderModel a filled in jujuProviderModel if able. First check
// the plan being used, then fall back to the JUJU_ environment variables,
// lastly check to see if an active juju can supply the data.
func getJujuProviderModel(ctx context.Context, planData jujuProviderModel) (jujuProviderModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	// If validation failed because we have both username/password
	// and client ID/secret combinations in the plan. Exit now.
	if planData.UserName.ValueString() != "" && planData.ClientID.ValueString() != "" {
		diags.AddError("Only username and password OR client id and "+
			"client secret may be used.",
			"Both username and client id are set in the plan. Please remove "+
				"one of the login methods and try again.")
		return planData, diags
	}

	// Check env vars to capture variables set outside of the plan.
	envVarData := jujuProviderModelEnvVar(diags)
	planEnvVarDataModel, planEnvVarDataDiags := planData.merge(envVarData, "environment variables")
	diags.Append(planEnvVarDataDiags...)
	if planEnvVarDataModel.valid() {
		return planEnvVarDataModel, diags
	}
	if planEnvVarDataModel.loginViaClientCredentials() {
		if planEnvVarDataModel.ControllerAddrs.ValueString() == "" {
			diags.AddError("Controller address required", "The provider must know which juju controller to use. Please add to plan or use the JUJU_CONTROLLER_ADDRESSES environment variable.")
		}
	}
	if diags.HasError() {
		return planEnvVarDataModel, diags
	}

	// Not all controller config contained in the plan, attempt
	// to find it via live discovery.
	liveData, cliAlive := jujuProviderModelLiveDiscovery()
	errMsgDataModel := planEnvVarDataModel
	if cliAlive {
		livePlanEnvVarDataModel, livePlanEnvVarDataDiags := planEnvVarDataModel.merge(liveData, "live discovery")
		diags.Append(livePlanEnvVarDataDiags...)
		if livePlanEnvVarDataModel.valid() {
			return livePlanEnvVarDataModel, diags
		}
		errMsgDataModel = livePlanEnvVarDataModel
	} else {
		tflog.Debug(ctx, "Live discovery of juju controller failed. The Juju CLI could not be accessed.")
	}

	// Validate controller config and return helpful error messages.
	if !errMsgDataModel.loginViaUsername() && !errMsgDataModel.loginViaClientCredentials() {
		diags.AddError(
			"Username and password or client id and client secret must be set",
			"Currently the provider can authenticate using username and password or client id and client secret, otherwise it will panic.",
		)
	}
	if errMsgDataModel.ControllerAddrs.ValueString() == "" {
		diags.AddError("Controller address required", "The provider must know which juju controller to use.")
	}
	if diags.HasError() {
		tflog.Debug(ctx, "Current login values.",
			map[string]interface{}{"jujuProviderModel": planData})
	}

	return errMsgDataModel, diags
}

// Resources returns a slice of functions to instantiate each Resource
// implementation.
//
// The resource type name is determined by the Resource implementing
// the Metadata method. All resources must have unique names.
func (p *jujuProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		func() resource.Resource { return NewControllerResource(p.newJujuCommand) },
		func() resource.Resource { return NewAccessModelResource() },
		func() resource.Resource { return NewAccessOfferResource() },
		func() resource.Resource { return NewApplicationResource() },
		func() resource.Resource { return NewCredentialResource() },
		func() resource.Resource { return NewIntegrationResource() },
		func() resource.Resource { return NewKubernetesCloudResource() },
		func() resource.Resource { return NewMachineResource() },
		func() resource.Resource { return NewModelResource() },
		func() resource.Resource { return NewOfferResource() },
		func() resource.Resource { return NewSSHKeyResource() },
		func() resource.Resource { return NewUserResource() },
		func() resource.Resource { return NewSecretResource() },
		func() resource.Resource { return NewAccessSecretResource() },
		func() resource.Resource { return NewJAASAccessModelResource() },
		func() resource.Resource { return NewJAASAccessCloudResource() },
		func() resource.Resource { return NewJAASAccessGroupResource() },
		func() resource.Resource { return NewJAASAccessRoleResource() },
		func() resource.Resource { return NewJAASAccessOfferResource() },
		func() resource.Resource { return NewJAASAccessControllerResource() },
		func() resource.Resource { return NewJAASGroupResource() },
		func() resource.Resource { return NewJAASRoleResource() },
		func() resource.Resource { return NewStoragePoolResource() },
		func() resource.Resource { return NewCloudResource() },
	}
}

// DataSources returns a slice of functions to instantiate each DataSource
// implementation.
//
// The data source type name is determined by the DataSource implementing
// the Metadata method. All data sources must have unique names.
func (p *jujuProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return NewApplicationDataSource() },
		func() datasource.DataSource { return NewMachineDataSource() },
		func() datasource.DataSource { return NewModelDataSource() },
		func() datasource.DataSource { return NewOfferDataSource() },
		func() datasource.DataSource { return NewSecretDataSource() },
		func() datasource.DataSource { return NewJAASGroupDataSource() },
		func() datasource.DataSource { return NewJAASRoleDataSource() },
		func() datasource.DataSource { return NewStoragePoolDataSource() },
	}
}

func checkClientErr(err error, config juju.ControllerConfiguration) diag.Diagnostics {
	var errDetail string
	var diags diag.Diagnostics

	x509error := &x509.UnknownAuthorityError{}
	x509HostError := &x509.HostnameError{}
	netOpError := &net.OpError{}
	if errors.As(err, x509error) || errors.As(err, x509HostError) {
		errDetail = "Verify the ca_certificate property set on the provider"

		if config.CACert == "" {
			errDetail = "The ca_certificate provider property is not set and the Juju certificate authority is not trusted by your system"
		}

		diags.AddError(x509error.Error(), errDetail)
		return diags
	}
	if errors.As(err, &netOpError) {
		errDetail = "Connection error, please check the controller_addresses property set on the provider"
		diags.AddError(netOpError.Error(), errDetail)
		return diags
	}
	diags.AddError("Client Error", err.Error())
	return diags
}

func checkControllerMode(diags diag.Diagnostics, config juju.Config, isControllerResource bool) diag.Diagnostics {
	if config.ControllerMode && !isControllerResource {
		diags.AddError("when controller_mode is true this resource cannot be used.", "")
		return diags
	} else if !config.ControllerMode && isControllerResource {
		diags.AddError("when controller_mode is false this resource cannot be used.", "")
	}
	return diags
}

// getProviderData extracts and validates provider data from a ConfigureRequest.
// It performs type assertion and controller mode validation in one step.
func getProviderData(req resource.ConfigureRequest, isControllerResource bool) (juju.ProviderData, diag.Diagnostics) {
	var diags diag.Diagnostics
	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		diags.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return juju.ProviderData{}, diags
	}
	diags = checkControllerMode(diags, provider.Config, isControllerResource)
	if diags.HasError() {
		return juju.ProviderData{}, diags
	}
	return provider, diags
}

// getProviderDataForDataSource extracts and validates provider data from a data source ConfigureRequest.
// It performs type assertion and controller mode validation in one step.
func getProviderDataForDataSource(req datasource.ConfigureRequest, isControllerResource bool) (juju.ProviderData, diag.Diagnostics) {
	var diags diag.Diagnostics
	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		diags.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return juju.ProviderData{}, diags
	}
	diags = checkControllerMode(diags, provider.Config, isControllerResource)
	if diags.HasError() {
		return juju.ProviderData{}, diags
	}
	return provider, diags
}
