package provider

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func New(version string) func() *schema.Provider {
	return func() *schema.Provider {
		p := &schema.Provider{
			Schema: map[string]*schema.Schema{},
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
