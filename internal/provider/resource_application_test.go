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
					resource.TestCheckResourceAttr("juju_application.this", "principal", "true"),
				),
			},
			{
				Config: testAccResourceApplicationConstraints(t, modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
				),
			},
			{
				Config: testAccResourceApplicationConstraintsSubordinate(t, modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
					resource.TestCheckResourceAttr("juju_application.this", "principal", "true"),
					resource.TestCheckResourceAttr("juju_application.subordinate", "principal", "false"),
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
				Config: testAccResourceApplicationUpdates(modelName, 1, 21, true, "machinename"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu"),
					resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "21"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "config.hostname", "machinename"),
				),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "21"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, false, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "expose.#", "0"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, 21, true, "machinename"),
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

func testAccResourceApplicationUpdates(modelName string, units int, revision int, expose bool, hostname string) string {
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
  config = {
	hostname = "%s"
  }
}
`, modelName, units, revision, exposeStr, hostname)
}

func testAccResourceApplicationConstraints(t *testing.T, modelName string, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  units = 0
  name = "test-app"
  charm {
    name     = "ubuntu"
    revision = 21
  }
  trust = true
  expose{}
  constraints = "%s"
}
`, modelName, constraints)
}

func testAccResourceApplicationConstraintsSubordinate(t *testing.T, modelName string, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  units = 0
  name = "test-app"
  charm {
    name     = "ubuntu"
    revision = 21
  }
  trust = true
  expose{}
  constraints = "%s"
}

resource "juju_application" "subordinate" {
	model = juju_model.this.name
	units = 0
	name = "test-subordinate"
	charm {
		name = "nrpe"
		revision = 96
	}
} 
`, modelName, constraints)
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