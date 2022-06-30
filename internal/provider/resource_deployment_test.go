package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

// TODO: test also for k8s substrate, tiny-bash charm is not supported
func TestAcc_ResourceDeployment(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-deployment")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceDeployment(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_deployment.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_deployment.this", "charm.0.name", "tiny-bash"),
				),
			},
		},
	})
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
