package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceOffer_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-offer")
	destModelName := acctest.RandomWithPrefix("tf-test-offer-dest")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOffer(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
				),
			},
			{
				Config: testAccResourceOfferXIntegration(modelName, destModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.that", "model", destModelName),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.that", "application.*", map[string]string{"name": "this", "endpoint": "db", "offer_url": ""}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.that", "application.*", map[string]string{"name": "", "endpoint": "", "offer_url": fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_offer.this",
			},
		},
	})
}

func testAccResourceOffer(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "db"
}
`, modelName)
}

func testAccResourceOfferXIntegration(srcModelName string, destModelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "db"
}

resource "juju_model" "that" {
	name = %q
}

resource "juju_application" "that" {
	model = juju_model.that.name
	name = "that"

	charm {
		name = "hello-juju"
		series = "focal"
	}
}

resource "juju_integration" "that" {
	model = juju_model.that.name

	application {
		name = juju_application.that.name
	}

	application {
		offer_url = juju_offer.this.url
	}
}
`, srcModelName, destModelName)
}
