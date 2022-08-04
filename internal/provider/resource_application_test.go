package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceApplication_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationBasic(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "tiny-bash"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.this",
			},
		},
	})
}

func TestAcc_ResourceApplication_Updates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationUpdates(modelName, 1, 19),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu"),
					resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "19"),
				),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 19),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 20),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "20"),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.this",
			},
		},
	})
}

func testAccResourceApplicationBasic(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  charm {
    name = "tiny-bash"
  }
  trust = true
}
`, modelName)
}

func testAccResourceApplicationUpdates(modelName string, units int, revision int) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  units = %d
  charm {
    name     = "ubuntu"
    revision = %d
  }
  trust = true
}
`, modelName, units, revision)
}
