package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceCharm(t *testing.T) {
	t.Skip("resource not yet implemented, remove this once you add your own code")

	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckCharmDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCharm,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("juju_model.development", "name", regexp.MustCompile("^development")),
					resource.TestMatchResourceAttr("juju_charm.postgres", "charm", regexp.MustCompile("^ch:postgres-k8s")),
				),
			},
		},
	})
}

func testAccCheckCharmDestroy(s *terraform.State) error {

	return nil
}

const testAccResourceCharm = `
resource "juju_model" "development" {
  name = "development"
}

resource "juju_model" "postgres" {
  model = juju_model.development.id
  charm = "ch:postgres-k8s"
}
`
