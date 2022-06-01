package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	t.Skip("data source not yet implemented, remove this once you add your own code")

	resource.UnitTest(t, resource.TestCase{
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
