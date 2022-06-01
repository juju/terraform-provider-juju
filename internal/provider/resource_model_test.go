package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceModel(t *testing.T) {
	t.Skip("resource not yet implemented, remove this once you add your own code")

	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckModelDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"juju_model.development", "name", regexp.MustCompile("^development")),
				),
			},
		},
	})
}

func testAccCheckModelDestroy(s *terraform.State) error {

	return nil
}

const testAccResourceModel = `
resource "juju_model" "development" {
  name = "development"
}
`
