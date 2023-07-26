package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/juju/terraform-provider-juju/version"
)

func TestAcc_ResourceOffer_sdk2_framework_migrate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-offer")
	destModelName := acctest.RandomWithPrefix("tf-test-offer-dest")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{

			{
				Config: testAccResourceOfferMigrate(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
				),
			},
			{
				Config: testAccResourceOfferXIntegrationMigrate(modelName, destModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.that", "model", destModelName),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.that", "application.*", map[string]string{"name": "this", "endpoint": "db", "offer_url": ""}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.that", "application.*", map[string]string{"name": "", "endpoint": "", "offer_url": fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")}),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_offer.this",
			},
		},
	})
}

func testAccResourceOfferMigrate(modelName string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this" {
    provider = oldjuju
	name = %q
}

resource "juju_application" "this" {
    provider = oldjuju
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_offer" "this" {
    provider = oldjuju
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "db"
}
`, modelName)
}

func testAccResourceOfferXIntegrationMigrate(srcModelName string, destModelName string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this" {
    provider = oldjuju
	name = %q
}

resource "juju_application" "this" {
    provider = oldjuju
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "postgresql"
		series = "focal"
	}
}

resource "juju_offer" "this" {
    provider = oldjuju
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "db"
}

resource "juju_model" "that" {
    provider = oldjuju
	name = %q
}

resource "juju_application" "that" {
    provider = oldjuju
	model = juju_model.that.name
	name = "that"

	charm {
		name = "hello-juju"
		series = "focal"
	}
}

resource "juju_integration" "that" {
    provider = oldjuju
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

func TestAcc_ResourceOffer_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-offer")

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: version.TerraformProviderJujuVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOfferStable(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
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

func testAccResourceOfferStable(modelName string) string {
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
