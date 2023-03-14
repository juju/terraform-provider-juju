package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceMachine(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func testAccDataSourceMachine(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}

resource "juju_machine" "machine" {
  model = juju_model.model.name
  name = "machine"
  series = "jammy"
}

data "juju_machine" "machine" {
  model = juju_model.model.name
  machine_id = juju_machine.machine.machine_id
}`, modelName)
}
