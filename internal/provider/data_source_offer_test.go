package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceOffer(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-offer-test-model")
	// ...-test-[0-9]+ is not a valid offer name, need to remove the dash before numbers
	offerName := fmt.Sprintf("tf-datasource-offer-test%d", acctest.RandInt())

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceOffer(t, modelName, offerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("data.juju_offer.this", "name", offerName),
				),
			},
		},
	})
}

func testAccDataSourceOffer(t *testing.T, modelName string, offerName string) string {
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
	name             = %q
}

data "juju_offer" "this" {
	url = juju_offer.this.url
}
`, modelName, offerName)
}
