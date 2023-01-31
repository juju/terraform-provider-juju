package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceIntegration(t *testing.T) {
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
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "two:db", "one:db")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "db"}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_integration.this",
			},
			{
				Config: testAccResourceIntegration(modelName, "three"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "three:db", "one:db")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "three", "endpoint": "db"}),
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
		name = "hello-juju"
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

resource "juju_application" "three" {
	model = juju_model.this.name
	name  = "three"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_integration" "this" {
	model = juju_model.this.name

	application {
		name = juju_application.one.name
	}

	application {
		name     = juju_application.%s.name
		endpoint = "db"
	}
}
`, modelName, integrationName)
}

func TestAcc_ResourceIntegrationWithViaCIDRs(t *testing.T) {
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")
	via := "127.0.0.1/32,127.0.0.3/32"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, dstModelName, via),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", srcModelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", srcModelName, "that:db", "this:db")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "this", "endpoint": "db"}),
					resource.TestCheckResourceAttr("juju_integration.this", "via", via),
				),
			},
		},
	})
}

func testAccResourceIntegrationWithVia(srcModelName string, dstModelName string, viaCIDRs string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_model" "that" {
	name = %q
}

resource "juju_application" "this" {
	model = juju_model.this.name
	name  = "this" 
	
	charm {
		name = "hello-juju"
		series = "focal"
	}
}

resource "juju_application" "that" {
	model = juju_model.that.name
	name  = "that"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_offer" "that" {
	model            = juju_model.that.name
	application_name = juju_application.that.name
	endpoint         = "db"
}

resource "juju_integration" "this" {
	model = juju_model.this.name
	via = %q

	application {
		name = juju_application.this.name
	}

	application {
		offer_url = juju_offer.that.url
	}
}
`, srcModelName, dstModelName, viaCIDRs)
}
