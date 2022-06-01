package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/juju/juju/osenv"
)

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{
				"controller": {
					Type:        schema.TypeString,
					Description: fmt.Sprintf("This is the Controller name be use with Juju provider and will default to `overlord`. This can also be set by the `%s` environment variable.", osenv.JujuControllerEnvKey),
					Optional:    true,
					DefaultFunc: schema.EnvDefaultFunc(osenv.JujuControllerEnvKey, ""),
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

		//p.ConfigureContextFunc = configure(version, p)

		return p
	}
}
