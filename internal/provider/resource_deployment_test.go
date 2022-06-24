package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceDeployment(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckDeploymentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceDeployment,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_deployment.model", "name", "development"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.name", "tiny-bash"),
				),
			},
		},
	})
}

func testAccCheckDeploymentDestroy(s *terraform.State) error {
	return nil
}

const testAccResourceDeployment = `
resource "juju_model" "development" {
  name = "development"
}

resource "juju_deployment" "this" {
  model = juju_model.development.name
  charm {
    name = "tiny-bash"
  }
}
`
