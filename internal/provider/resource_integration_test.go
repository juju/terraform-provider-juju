package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceIntegration(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName, "two"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "two:db-admin", "one:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "backend-db-admin"}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_integration.this",
			},
			{
				Config: testAccResourceIntegration(modelName, "two"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "two:db-admin", "one:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
				),
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	return nil
}

func testAccResourceIntegration(modelName string, integrationName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model = juju_model.this.name
	name  = "one" 
	
	charm {
		name = "pgbouncer"
		series = "focal"
	}
}

resource "juju_application" "two" {
	model = juju_model.this.name
	name  = "two"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_integration" "this" {
	model = juju_model.this.name

	application {
		name     = juju_application.%s.name
		endpoint = "db-admin"
	}

	application {
		name = juju_application.one.name
		endpoint = "backend-db-admin"
	}
}
`, modelName, integrationName)
}

func TestAcc_ResourceIntegrationWithViaCIDRs(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")
	// srcModelName := "modela"
	// dstModelName := "modelb"
	via := "127.0.0.1/32,127.0.0.3/32"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, dstModelName, via),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.a", "model", srcModelName),
					resource.TestCheckResourceAttr("juju_integration.a", "id", fmt.Sprintf("%v:%v:%v", srcModelName, "a:db-admin", "b:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.a", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.a", "application.*", map[string]string{"name": "a", "endpoint": "db-admin"}),
					resource.TestCheckResourceAttr("juju_integration.a", "via", via),
				),
			},
		},
	})
}

// testAccResourceIntegrationWithVia generates a plan where a
// postgresql:db-admin relates to a pgbouncer:backend-db-admin using
// and offer of pgbouncer.
func testAccResourceIntegrationWithVia(srcModelName string, dstModelName string, viaCIDRs string) string {
	return fmt.Sprintf(`
resource "juju_model" "a" {
	name = %q
}

resource "juju_application" "a" {
	model = juju_model.a.name
	name  = "a" 
	
	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_model" "b" {
	name = %q
}

resource "juju_application" "b" {
	model = juju_model.b.name
	name  = "b"
	
	charm {
		name = "pgbouncer"
		series = "focal"
	}
}

resource "juju_offer" "b" {
	model            = juju_model.b.name
	application_name = juju_application.b.name
	endpoint         = "backend-db-admin"
}

resource "juju_integration" "a" {
	model = juju_model.a.name
	via = %q

	application {
		name = juju_application.a.name
		endpoint = "db-admin"
	}
	
	application {
		offer_url = juju_offer.b.url
	}
}
`, srcModelName, dstModelName, viaCIDRs)
}
