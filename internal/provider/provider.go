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
	"strings"

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
	JujuControllerEnvKey   = "JUJU_CONTROLLER_ADDRESSES"
	JujuUsernameEnvKey     = "JUJU_USERNAME"
	JujuPasswordEnvKey     = "JUJU_PASSWORD"
	JujuCACertEnvKey       = "JUJU_CA_CERT"
	JujuClientIDEnvKey     = "JUJU_CLIENT_ID"
	JujuClientSecretEnvKey = "JUJU_CLIENT_SECRET"

	JujuController   = "controller_addresses"
	JujuUsername     = "username"
	JujuPassword     = "password"
	JujuClientID     = "client_id"
	JujuClientSecret = "client_secret"
	JujuCACert       = "ca_certificate"

	TwoSourcesAuthWarning = "Two sources of identity for controller login"
)

// jujuProviderModelEnvVar gets the controller config,
// from environment variables.
func jujuProviderModelEnvVar() jujuProviderModel {
	return jujuProviderModel{
		ControllerAddrs: getEnvVar(JujuControllerEnvKey),
		CACert:          getEnvVar(JujuCACertEnvKey),
		ClientID:        getEnvVar(JujuClientIDEnvKey),
		ClientSecret:    getEnvVar(JujuClientSecretEnvKey),
		UserName:        getEnvVar(JujuUsernameEnvKey),
		Password:        getEnvVar(JujuPasswordEnvKey),
	}
}

func jujuProviderModelLiveDiscovery() (jujuProviderModel, bool) {
	data := jujuProviderModel{}
	controllerConfig, cliNotExist := juju.GetLocalControllerConfig()

	if ctrlAddrs, ok := controllerConfig[JujuControllerEnvKey]; ok && ctrlAddrs != "" {
		data.ControllerAddrs = types.StringValue(ctrlAddrs)
	}
	if caCert, ok := controllerConfig[JujuCACertEnvKey]; ok && caCert != "" {
		data.ControllerAddrs = types.StringValue(caCert)
	}
	if user, ok := controllerConfig[JujuUsernameEnvKey]; ok && user != "" {
		data.ControllerAddrs = types.StringValue(user)
	}
	if password, ok := controllerConfig[JujuPasswordEnvKey]; ok && password != "" {
		data.ControllerAddrs = types.StringValue(password)
	}
	return data, cliNotExist
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

// NewJujuProvider returns a framework style terraform provider.
func NewJujuProvider(version string) provider.Provider {
	return &jujuProvider{version: version}
}

type jujuProvider struct {
	version string
}

type jujuProviderModel struct {
	ControllerAddrs types.String `tfsdk:"controller_addresses"`
	UserName        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	CACert          types.String `tfsdk:"ca_certificate"`
	ClientID        types.String `tfsdk:"client_id"`
	ClientSecret    types.String `tfsdk:"client_secret"`
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
		j.CACert.ValueString() != "" &&
		(validUserPass || validClientCredentials) &&
		!(validUserPass && validClientCredentials)
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
			JujuController: schema.StringAttribute{
				Description: fmt.Sprintf("This is the Controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
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
				Description: fmt.Sprintf("This is the client ID to be used. This can also be set by the `%s` environment variable", JujuClientIDEnvKey),
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(JujuUsername),
						path.MatchRoot(JujuPassword),
					}...),
				},
			},
			JujuClientSecret: schema.StringAttribute{
				Description: fmt.Sprintf("This is the client secret to be used. This can also be set by the `%s` environment variable", JujuClientSecretEnvKey),
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
				Description: fmt.Sprintf("This is the certificate to use for identification. This can also be set by the `%s` environment variable", JujuCACertEnvKey),
				Optional:    true,
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
	// Get data required for configuring the juju client.
	data, diags := getJujuProviderModel(ctx, req)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	config := juju.ControllerConfiguration{
		ControllerAddresses: strings.Split(data.ControllerAddrs.ValueString(), ","),
		Username:            data.UserName.ValueString(),
		Password:            data.Password.ValueString(),
		CACert:              data.CACert.ValueString(),
		ClientID:            data.ClientID.ValueString(),
		ClientSecret:        data.ClientSecret.ValueString(),
	}
	client, err := juju.NewClient(ctx, config)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create juju client, got error: %s", err))
		return
	}

	// Here we are testing that we can connect successfully to the Juju server
	// this prevents having logic to check the connection is OK in every function
	testConn, err := client.Models.GetConnection(nil)
	if err != nil {
		resp.Diagnostics.Append(checkClientErr(err, config)...)
		return
	}
	_ = testConn.Close()

	resp.ResourceData = client
	resp.DataSourceData = client
}

// getJujuProviderModel a filled in jujuProviderModel if able. First check
// the plan being used, then fall back to the JUJU_ environment variables,
// lastly check to see if an active juju can supply the data.
func getJujuProviderModel(ctx context.Context, req provider.ConfigureRequest) (jujuProviderModel, diag.Diagnostics) {
	var planData jujuProviderModel
	var diags diag.Diagnostics

	// Read Terraform configuration data into the juju provider model.
	diags.Append(req.Config.Get(ctx, &planData)...)
	if diags.HasError() {
		return planData, diags
	}
	if planData.valid() {
		// The plan contained full controller config,
		// no need to continue
		return planData, diags
	}
	// If validation failed because we have both username/password
	// and client ID/secret combinations in the plan. Exit now.
	if planData.UserName.ValueString() != "" && planData.ClientID.ValueString() != "" {
		diags.AddError("Only username and password OR client id and "+
			"client secret may be used.",
			"Both username and client id are set in the plan. Please remove "+
				"one of the login methods and try again.")
		return planData, diags
	}

	// Not all controller config contained in the plan, attempt
	// to find it via the optional environment variables.
	envVarData := jujuProviderModelEnvVar()
	planEnvVarDataModel, planEnvVarDataDiags := planData.merge(envVarData, "environment variables")
	diags.Append(planEnvVarDataDiags...)
	if planEnvVarDataModel.valid() {
		return planEnvVarDataModel, diags
	}
	if planEnvVarDataModel.loginViaClientCredentials() {
		if planEnvVarDataModel.ControllerAddrs.ValueString() == "" {
			diags.AddError("Controller address required", "The provider must know which juju controller to use. Please add to plan or use the JUJU_CONTROLLER_ADDRESSES environment variable.")
		}
		if planEnvVarDataModel.CACert.ValueString() == "" {
			diags.AddError("Controller CACert required", "For the Juju certificate authority to be trusted by your system. Please add to plan or use the JUJU_CA_CERT environment variable.")
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
	if errMsgDataModel.CACert.ValueString() == "" {
		diags.AddError("Controller CACert required", "For the Juju certificate authority to be trusted by your system.")
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
		func() resource.Resource { return NewAccessModelResource() },
		func() resource.Resource { return NewApplicationResource() },
		func() resource.Resource { return NewCredentialResource() },
		func() resource.Resource { return NewIntegrationResource() },
		func() resource.Resource { return NewMachineResource() },
		func() resource.Resource { return NewModelResource() },
		func() resource.Resource { return NewOfferResource() },
		func() resource.Resource { return NewSSHKeyResource() },
		func() resource.Resource { return NewUserResource() },
		func() resource.Resource { return NewSecretResource() },
		func() resource.Resource { return NewAccessSecretResource() },
	}
}

// DataSources returns a slice of functions to instantiate each DataSource
// implementation.
//
// The data source type name is determined by the DataSource implementing
// the Metadata method. All data sources must have unique names.
func (p *jujuProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		func() datasource.DataSource { return NewMachineDataSource() },
		func() datasource.DataSource { return NewModelDataSource() },
		func() datasource.DataSource { return NewOfferDataSource() },
		func() datasource.DataSource { return NewSecretDataSource() },
	}
}

func checkClientErr(err error, config juju.ControllerConfiguration) diag.Diagnostics {
	var errDetail string
	var diags diag.Diagnostics

	x509error := &x509.UnknownAuthorityError{}
	netOpError := &net.OpError{}
	if errors.As(err, x509error) {
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
