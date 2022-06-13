package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/juju/osenv"
	"github.com/juju/terraform-provider-juju/internal/client"
)

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"controller": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the Controller address to connect to, defaults to localhost:17070. This can also be set by the `%s` environment variable.", osenv.JujuControllerEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(osenv.JujuControllerEnvKey, "localhost:17070"),
				},
				"username": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the username registered with the controller to be used. This can also be set by the `JUJU_USERNAME` environment variable"),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("JUJU_USERNAME", nil),
				},
				"password": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the password of the username to be used. This can also be set by the `JUJU_PASSWORD` environment variable"),
					Optional:    true,
					Sensitive:   true,
					DefaultFunc: schema.EnvDefaultFunc("JUJU_PASSWORD", nil),
				},
				"ca_certificate": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the certificate to use for identification. This can also be set by the `JUJU_CA_CERT` environment variable"),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc("JUJU_CA_CERT", nil),
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

		controllerAddress := d.Get("controller").(string)
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

		simpleConfig := connector.SimpleConfig{
			ControllerAddresses: []string{controllerAddress},
			Username:            username,
			Password:            password,
			CACert:              caCert,
		}

		internalClient, err := client.NewClient(simpleConfig)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		return &internalClient, diags
	}
}
