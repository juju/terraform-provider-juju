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
					resource.TestCheckResourceAttr("juju_application.this", "name", "test-app"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
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
				Config: testAccResourceApplicationUpdates(modelName, 1, 21, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu"),
					resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "21"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
				),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "21"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, false),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "expose.#", "0"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.this",
			},
		},
	})
}

func TestAcc_ResourceApplication_Placement(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationPlacement(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.placement", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.placement", "name", "hello-juju"),
					resource.TestCheckResourceAttr("juju_application.placement", "charm.0.name", "hello-juju"),
					resource.TestCheckResourceAttr("juju_application.placement", "placement", "0"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.placement",
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
  name = "test-app"
  charm {
    name = "ubuntu"
  }
  trust = true
  expose{}
}
`, modelName)
}

func testAccResourceApplicationUpdates(modelName string, units int, revision int, expose bool) string {
	exposeStr := "expose{}"
	if !expose {
		exposeStr = ""
	}
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  units = %d
  name = "test-app"
  charm {
    name     = "ubuntu"
    revision = %d
  }
  trust = true
  %s
}
`, modelName, units, revision, exposeStr)
}
 
func testAccResourceApplicationPlacement(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	series = "focal"
	model = juju_model.this.name
}

resource "juju_application" "placement" {
	model = juju_model.this.name
	units = 1
	name = "hello-juju"

	charm {
		name = "hello-juju"
	}

	placement = split(":", juju_machine.this.id)[1]
}
`, modelName)
}