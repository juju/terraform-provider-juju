package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// TODO: test also for k8s substrate, tiny-bash charm is not supported
func TestAcc_ResourceDeployment(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-deployment")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckDeploymentDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceDeployment(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_deployment.this", "name", modelName),
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

func testAccResourceDeployment(modelName string) string {
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
