package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-model-test")

	resource.Test(t, resource.TestCase{
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceModel(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_model.model", "name", modelName),
				),
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: "0.8.0",
						Source:            "juju/juju",
					},
				},
				PreConfig: func() { testAccPreCheck(t) },
			},
			{
				Config: testAccFrameworkDataSourceModel(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_model.model", "name", modelName),
					resource.TestCheckResourceAttrSet("data.juju_model.model", "uuid"),
				),
				ProtoV5ProviderFactories: map[string]func() (tfprotov5.ProviderServer, error){
					"juju": providerserver.NewProtocol5WithError(NewJujuProvider("dev")),
					"oldjuju": func() (tfprotov5.ProviderServer, error) {
						return schema.NewGRPCProviderServer(New("dev")()), nil
					},
				},
			},
		},
	})
}

func testAccDataSourceModel(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
	name = %q
}

data "juju_model" "model" {
	name = juju_model.model.name
}`, modelName)
}

func testAccFrameworkDataSourceModel(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
provider "juju" {}

provider "oldjuju" {}

resource "juju_model" "model" {
	provider = oldjuju
	name = %q
}

data "juju_model" "model" {
	provider = juju
	name = juju_model.model.name
}`, modelName)
}
