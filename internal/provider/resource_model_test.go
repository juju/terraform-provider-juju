package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"juju_model.model", "name", regexp.MustCompile("^"+modelName+"$")),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     modelName,
				ResourceName:      "juju_model.model",
			},
		},
	})
}

func testAccResourceModel(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}`, modelName)
}
