package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceMachine_sdk2_framework_migrate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine_sdk2_framework_migrate(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func testAccDataSourceMachine_sdk2_framework_migrate(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "model" {
  provider = oldjuju
  name = %q
}

resource "juju_machine" "machine" {
  provider = oldjuju
  model = juju_model.model.name
  name = "machine"
  series = "jammy"
}

data "juju_machine" "machine" {
  provider = oldjuju
  model = juju_model.model.name
  machine_id = juju_machine.machine.machine_id
}`, modelName)
}

func TestAcc_DataSourceMachine_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine_Stable(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func testAccDataSourceMachine_Stable(t *testing.T, modelName string) string {
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
