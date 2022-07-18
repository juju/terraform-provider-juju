package provider

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

const (
	JujuControllerEnvKey = "JUJU_CONTROLLER_ADDRESSES"
	JujuUsernameEnvKey   = "JUJU_USERNAME"
	JujuPasswordEnvKey   = "JUJU_PASSWORD"
	JujuCACertEnvKey     = "JUJU_CA_CERT"
)

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"controller_addresses": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the Controller addresses to connect to, defaults to localhost:17070, multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(JujuControllerEnvKey, "localhost:17070"),
				},
				"username": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the username registered with the controller to be used. This can also be set by the `%s` environment variable", JujuUsernameEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(JujuUsernameEnvKey, nil),
				},
				"password": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the password of the username to be used. This can also be set by the `%s` environment variable", JujuPasswordEnvKey),
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc(JujuPasswordEnvKey, nil),
				},
				"ca_certificate": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the certificate to use for identification. This can also be set by the `%s` environment variable", JujuCACertEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(JujuCACertEnvKey, nil),
				},
			},
			DataSourcesMap: map[string]*schema.Resource{
				"juju_model": dataSourceModel(),
			},
			ResourcesMap: map[string]*schema.Resource{
				"juju_model":       resourceModel(),
				"juju_application": resourceApplication(),
				"juju_integration": resourceIntegration(),
			},
		}

		p.ConfigureContextFunc = configure(version, p)

		return p
	}
}

func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
		var diags diag.Diagnostics

		ControllerAddresses := strings.Split(d.Get("controller_addresses").(string), ",")
		username := d.Get("username").(string)
		password := d.Get("password").(string)
		caCert := d.Get("ca_certificate").(string)

		//TODO: remove this check when other auth methods are added
		if username == "" || password == "" {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Username and password must be set",
				Detail:   "Currently the provider can only authenticate using username and password based authentication, if both are empty the provider will panic",
			})
			return nil, diags
		}

		config := juju.Configuration{
			ControllerAddresses: ControllerAddresses,
			Username:            username,
			Password:            password,
			CACert:              caCert,
		}
		client, err := juju.NewClient(config)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		// Here we are testing that we can connect successfully to the Juju server
		// this prevents having logic to check the connection is OK in every function
		testConn, err := client.Models.GetConnection(nil)
		if err != nil {
			return nil, checkClientErr(err, diags, config)
		}
		testConn.Close()

		return client, diags
	}
}

func checkClientErr(err error, diags diag.Diagnostics, config juju.Configuration) diag.Diagnostics {
	var errDetail string

	x509error := &x509.UnknownAuthorityError{}
	netOpError := &net.OpError{}
	if errors.As(err, x509error) {
		errDetail = "Verify the ca_certificate property set on the provider"

		if config.CACert == "" {
			errDetail = "The ca_certificate provider property is not set and the Juju certificate authority is not trusted by your system"
		}

		return append(diags, diag.Diagnostic{
			Summary: x509error.Error(),
			Detail:  errDetail,
		})
	}
	if errors.As(err, &netOpError) {
		errDetail = "Connection error, please check the controller_addresses property set on the provider"

		return append(diags, diag.Diagnostic{
			Summary: netOpError.Error(),
			Detail:  errDetail,
		})
	}
	return diag.FromErr(err)
}
