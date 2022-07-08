package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceIntegration(t *testing.T) {
	// TODO: remove once other operations are implemented
	t.Skip("skipped until delete operation is implemented")
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:db", "two:db")),
				),
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	return nil
}

func testAccResourceIntegration(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_deployment" "one" {
	model = juju_model.this.name
	name  = "one" 
	
	charm {
		name = "hello-juju"
	}
}

resource "juju_deployment" "two" {
	model = juju_model.this.name
	name  = "two"

	charm {
		name = "postgresql"
	}
}

resource "juju_integration" "this" {
	model = juju_model.this.name

	application {
		name = juju_deployment.one.name
	}

	application {
		name     = juju_deployment.two.name
		endpoint = "db"
	}
}
`, modelName)
}
