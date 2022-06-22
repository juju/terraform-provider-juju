package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-model-test")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceModel(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"data.juju_model.model", "name", regexp.MustCompile("^"+modelName+"$")),
				),
			},
		},
	})
}

func testAccDataSourceModel(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
	name = %q
}

data "juju_model" "model" {
  name = juju_model.model.name
}`, modelName)
}
