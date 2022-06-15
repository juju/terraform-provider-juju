package provider

import (
	"context"
	"fmt"
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
					Description: fmt.Sprintf("This is the Controller addresses to connect to, defaults to ['localhost:17070'], multiple addresses can be provided in this format: ['<host>:<port>', '<host>:<port>', ...]. This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(JujuControllerEnvKey, "['localhost:17070']"),
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
				"juju_model":    resourceModel(),
				"juju_charm":    resourceCharm(),
				"juju_relation": resourceRelation(),
			},
		}

		p.ConfigureContextFunc = configure(version, p)

		return p
	}
}

func configure(version string, p *schema.Provider) func(context.Context, *schema.ResourceData) (interface{}, diag.Diagnostics) {
	return func(_ context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {

		var diags diag.Diagnostics

		ControllerAddresses := d.Get("controller_addresses").(string)
		ControllerAddresses = strings.Replace(ControllerAddresses, "[", "", -1)
		ControllerAddresses = strings.Replace(ControllerAddresses, "]", "", -1)
		ControllerAddresses = strings.Replace(ControllerAddresses, "'", "", -1)
		ControllerAddressesParsed := strings.Split(ControllerAddresses, ",")
		username := d.Get("username").(string)
		password := d.Get("password").(string)
		caCert := d.Get("ca_certificate").(string)

		if !((username != "" && password != "") || (username == "" && password == "")) {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Username and password must both be present or both should be omitted",
				Detail:   "If only one of username or password is defined, the provider will not be able to authenticate via the credentials method",
			})
		}

		config := juju.Configuration{
			ControllerAddresses: ControllerAddressesParsed,
			Username:            username,
			Password:            password,
			CACert:              caCert,
		}
		client, err := juju.NewClient(config)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		return client, diags
	}
}
