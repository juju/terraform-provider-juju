package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	// NOTE: comment this out when running locally
	t.Skip("test automation not yet implemented, avoid running on GitHub Actions")

	// NOTE: requires `juju create-model development` before executing
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceModel,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"data.juju_model.development", "name", regexp.MustCompile("^development")),
				),
			},
		},
	})
}

const testAccDataSourceModel = `
data "juju_model" "development" {
  name = "development"
}
`
