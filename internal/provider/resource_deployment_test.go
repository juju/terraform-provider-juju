package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceDeployment_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-deployment")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceDeploymentBasic(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_deployment.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.name", "tiny-bash"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_deployment.this",
			},
		},
	})
}

func TestAcc_ResourceDeployment_Updates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-deployment")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceDeploymentUpdates(modelName, 1, 19),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_deployment.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.name", "ubuntu"),
					resource.TestCheckResourceAttr("juju_deployment.this", "units", "1"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.revision", "19"),
				),
			},
			{
				Config: testAccResourceDeploymentUpdates(modelName, 2, 19),
				Check:  resource.TestCheckResourceAttr("juju_deployment.this", "units", "2"),
			},
			{
				Config: testAccResourceDeploymentUpdates(modelName, 2, 20),
				Check:  resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.revision", "20"),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_deployment.this",
			},
		},
	})
}

func testAccResourceDeploymentBasic(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_deployment" "this" {
  model = juju_model.this.name
  charm {
    name = "tiny-bash"
  }
}
`, modelName)
}

func testAccResourceDeploymentUpdates(modelName string, units int, revision int) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_deployment" "this" {
  model = juju_model.this.name
  units = %d
  charm {
    name     = "ubuntu"
    revision = %d
  }
}
`, modelName, units, revision)
}
