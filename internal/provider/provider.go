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

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	frameworkschema "github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

const (
	JujuControllerEnvKey = "JUJU_CONTROLLER_ADDRESSES"
	JujuUsernameEnvKey   = "JUJU_USERNAME"
	JujuPasswordEnvKey   = "JUJU_PASSWORD"
	JujuCACertEnvKey     = "JUJU_CA_CERT"

	JujuController = "controller_addresses"
	JujuUsername   = "username"
	JujuPassword   = "password"
	JujuCACert     = "ca_certificate"
)

// New returns an sdk2 style terraform provider.
func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				JujuController: {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the Controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
					Optional:    true,
				},
				JujuUsername: {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the username registered with the controller to be used. This can also be set by the `%s` environment variable", JujuUsernameEnvKey),
					Optional:    true,
				},
				JujuPassword: {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the password of the username to be used. This can also be set by the `%s` environment variable", JujuPasswordEnvKey),
					Optional:    true,
					Sensitive:   true,
				},
				JujuCACert: {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the certificate to use for identification. This can also be set by the `%s` environment variable", JujuCACertEnvKey),
					Optional:    true,
				},
			},
			ResourcesMap: map[string]*schema.Resource{},
		}
		p.ConfigureContextFunc = configure()

		return p
	}
}

func configure() func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		var diags diag.Diagnostics

		controllerAddresses := strings.Split(d.Get(JujuController).(string), ",")
		username := d.Get(JujuUsername).(string)
		password := d.Get(JujuPassword).(string)
		caCert := d.Get(JujuCACert).(string)

		if (len(controllerAddresses) == 1 && controllerAddresses[0] == "") ||
			username == "" || password == "" || caCert == "" {
			// Look for any config data not directly supplied in
			// the plan.
			liveData, err := populateJujuProviderModelLive()
			if err != nil {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.Error,
					Summary:  "Gather live client data",
					Detail:   err.Error(),
				})
				return nil, diags
			}
			if len(controllerAddresses) == 1 && controllerAddresses[0] == "" {
				controllerAddresses = strings.Split(liveData.ControllerAddrs.ValueString(), ",")
			}
			if username == "" {
				username = liveData.UserName.ValueString()
			}
			if password == "" {
				password = liveData.Password.ValueString()
			}
			if caCert == "" {
				caCert = liveData.CACert.ValueString()
			}
		}

		// Validate the controller config.
		if username == "" || password == "" {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Username and password must be set",
				Detail:   "Currently the provider can only authenticate using username and password based authentication, if both are empty the provider will panic",
			})
		}
		if len(controllerAddresses) > 1 && controllerAddresses[0] == "" {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Controller address required",
				Detail:   "The provider must know which juju controller to use.",
			})
		}
		if caCert == "" {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Controller CACert",
				Detail:   "Required for the Juju certificate authority to be trusted by your system",
			})
		}

		if diags.HasError() {
			return nil, diags
		}

		config := juju.Configuration{
			ControllerAddresses: controllerAddresses,
			Username:            username,
			Password:            password,
			CACert:              caCert,
		}
		client, err := juju.NewClient(ctx, config)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		// Here we are testing that we can connect successfully to the Juju server
		// this prevents having logic to check the connection is OK in every function
		testConn, err := client.Models.GetConnection(nil)
		if err != nil {
			for _, v := range checkClientErr(err, config) {
				diags = append(diags, diag.Diagnostic{
					Summary: v.Summary(),
					Detail:  v.Detail(),
				})
			}
			return nil, diags
		}
		_ = testConn.Close()

		return client, diags
	}
}

// populateJujuProviderModelLive gets the controller config,
// first from environment variables, then from a live juju
// controller as a fallback.
func populateJujuProviderModelLive() (jujuProviderModel, error) {
	data := jujuProviderModel{}
	controllerConfig, err := juju.GetLocalControllerConfig()
	if err != nil {
		return data, err
	}

	data.ControllerAddrs = types.StringValue(getField(JujuControllerEnvKey, controllerConfig))
	data.UserName = types.StringValue(getField(JujuUsernameEnvKey, controllerConfig))
	data.Password = types.StringValue(getField(JujuPasswordEnvKey, controllerConfig))
	data.CACert = types.StringValue(getField(JujuCACertEnvKey, controllerConfig))

	return data, nil
}

func getField(field string, config map[string]string) string {
	// get the value from the environment variable
	controller := os.Getenv(field)
	if controller == "" {
		// fall back to the live juju data
		controller = config[field]
	}
	return controller
}

// Ensure jujuProvider satisfies various provider interfaces.
var _ frameworkprovider.Provider = &jujuProvider{}

// NewJujuProvider returns a framework style terraform provider.
func NewJujuProvider(version string) frameworkprovider.Provider {
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
}

func (j jujuProviderModel) valid() bool {
	return j.ControllerAddrs.ValueString() != "" &&
		j.UserName.ValueString() != "" &&
		j.Password.ValueString() != "" &&
		j.CACert.ValueString() != ""
}

// Metadata returns the metadata for the provider, such as
// a type name and version data.
func (p *jujuProvider) Metadata(_ context.Context, _ frameworkprovider.MetadataRequest, resp *frameworkprovider.MetadataResponse) {
	resp.TypeName = "juju"
	resp.Version = p.version
}

// Schema returns the schema for this provider, specifically
// it defines the juju controller config necessary to create
// a juju client.
func (p *jujuProvider) Schema(_ context.Context, _ frameworkprovider.SchemaRequest, resp *frameworkprovider.SchemaResponse) {
	resp.Schema = frameworkschema.Schema{
		Attributes: map[string]frameworkschema.Attribute{
			JujuController: frameworkschema.StringAttribute{
				Description: fmt.Sprintf("This is the Controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
				Optional:    true,
			},
			JujuUsername: frameworkschema.StringAttribute{
				Description: fmt.Sprintf("This is the username registered with the controller to be used. This can also be set by the `%s` environment variable", JujuUsernameEnvKey),
				Optional:    true,
			},
			JujuPassword: frameworkschema.StringAttribute{
				Description: fmt.Sprintf("This is the password of the username to be used. This can also be set by the `%s` environment variable", JujuPasswordEnvKey),
				Optional:    true,
				Sensitive:   true,
			},
			JujuCACert: frameworkschema.StringAttribute{
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
func (p *jujuProvider) Configure(ctx context.Context, req frameworkprovider.ConfigureRequest, resp *frameworkprovider.ConfigureResponse) {
	// Get data required for configuring the juju client.
	data, diags := getJujuProviderModel(ctx, req)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	config := juju.Configuration{
		ControllerAddresses: strings.Split(data.ControllerAddrs.ValueString(), ","),
		Username:            data.UserName.ValueString(),
		Password:            data.Password.ValueString(),
		CACert:              data.CACert.ValueString(),
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
func getJujuProviderModel(ctx context.Context, req frameworkprovider.ConfigureRequest) (jujuProviderModel, frameworkdiag.Diagnostics) {
	var data jujuProviderModel
	var diags frameworkdiag.Diagnostics

	// Read Terraform configuration data into the data model
	diags.Append(req.Config.Get(ctx, &data)...)
	if diags.HasError() {
		return data, diags
	}
	if data.valid() {
		// The plan contained full controller config,
		// no need to continue
		return data, diags
	}

	// Not all controller config contained in the plan, attempt
	// to find it.
	liveData, err := populateJujuProviderModelLive()
	if err != nil {
		diags.AddError("Unable to get live controller data", err.Error())
		return data, diags
	}
	if data.ControllerAddrs.ValueString() == "" {
		data.ControllerAddrs = liveData.ControllerAddrs
	}
	if data.UserName.ValueString() == "" {
		data.UserName = liveData.UserName
	}
	if data.Password.ValueString() == "" {
		data.Password = liveData.Password
	}
	if data.CACert.ValueString() == "" {
		data.CACert = liveData.CACert
	}

	// Validate controller config and return helpful error messages.
	if data.UserName.ValueString() == "" || data.Password.ValueString() == "" {
		diags.AddError("Username and password must be set", "Currently the provider can only authenticate using username and password based authentication, if both are empty the provider will panic")
	}

	if data.ControllerAddrs.ValueString() == "" {
		diags.AddError("Controller address required", "The provider must know which juju controller to use.")
	}

	if data.CACert.ValueString() == "" {
		diags.AddError("Controller CACert", "Required for the Juju certificate authority to be trusted by your system")
	}

	return data, diags
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
	}
}

func checkClientErr(err error, config juju.Configuration) frameworkdiag.Diagnostics {
	var errDetail string
	var diags frameworkdiag.Diagnostics

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
