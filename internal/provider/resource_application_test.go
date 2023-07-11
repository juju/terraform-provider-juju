package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceApplication_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"
	appInvalidName := "test_app"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceApplicationBasic(modelName, appInvalidName),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: invalid application name \"%s\", unexpected character _", appInvalidName)),
			},
			{
				Config: testAccResourceApplicationBasic(modelName, appName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "jameinel-ubuntu-lite"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "principal", "true"),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					// cores constraint is not valid in K8s
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationConstraints(t, modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
				),
			},
			{
				// specific constraints for k8s
				SkipFunc: func() (bool, error) {
					return testingCloud != MicroK8sTesting, nil
				},
				Config: testAccResourceApplicationConstraints(t, modelName, "arch=amd64 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 mem=4096M"),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					// skip if we are not in lxd environment
					return testingCloud != LXDCloudTesting, nil
				},
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
	appName := "jameinel-ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		appName = "hello-kubecon"
	}
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationUpdates(modelName, 1, true, "machinename"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
					// (juanmanuel-tirado) Uncomment and test when running
					// a different charm with other config
					//resource.TestCheckResourceAttr("juju_application.this", "config.hostname", "machinename"),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationUpdates(modelName, 2, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationUpdates(modelName, 2, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "10"),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != MicroK8sTesting, nil
				},
				Config: testAccResourceApplicationUpdates(modelName, 2, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "19"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, false, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "expose.#", "0"),
			},
			{
				Config: testAccResourceApplicationUpdates(modelName, 2, true, "machinename"),
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

func TestAcc_CharmUpdates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationUpdatesCharm(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "latest/stable"),
				),
			},
			{
				// move to latest/edge
				Config: testAccResourceApplicationUpdatesCharm(modelName, "latest/edge"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "latest/edge"),
				),
			},
			{
				// move back to latest/stable
				Config: testAccResourceApplicationUpdatesCharm(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "latest/stable"),
				),
			},
		},
	})
}

func testAccResourceApplicationBasic(modelName, appInvalidName string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = %q
		  charm {
			name = "jameinel-ubuntu-lite"
		  }
		  trust = true
		  expose{}
		}
		`, modelName, appInvalidName)
	} else {
		// if we have a K8s deployment we need the machine hostname
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = %q
		  charm {
			name = "jameinel-ubuntu-lite"
		  }
		  trust = true
		  expose{}
		  config = {
			juju-external-hostname="myhostname"
		  }
		}
		`, modelName, appInvalidName)
	}
}

func testAccResourceApplicationUpdates(modelName string, units int, expose bool, hostname string) string {
	exposeStr := "expose{}"
	if !expose {
		exposeStr = ""
	}

	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  units = %d
		  name = "test-app"
		  charm {
			name     = "jameinel-ubuntu-lite"
		  }
		  trust = true
		  %s
		  # config = {
		  #	 hostname = "%s"
		  # }
		}
		`, modelName, units, exposeStr, hostname)
	} else {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  units = %d
		  name = "test-app"
		  charm {
			name     = "hello-kubecon"
		  }
		  trust = true
		  %s
		  config = {
		  	# hostname = "%s"
			juju-external-hostname="myhostname"
		  }
		}
		`, modelName, units, exposeStr, hostname)
	}
}

func testAccResourceApplicationUpdatesCharm(modelName string, channel string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = "test-app"
		  charm {
			name     = "ubuntu"
			channel = %q
		  }
		}
		`, modelName, channel)
	} else {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = "test-app"
		  charm {
			name     = "hello-kubecon"
			channel = %q
		  }
		}
		`, modelName, channel)
	}
}

// testAccResourceApplicationConstraints will return two set for constraint
// applications. The version to be used in K8s sets the juju-external-hostname
// because we set the expose parameter.
func testAccResourceApplicationConstraints(t *testing.T, modelName string, constraints string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  units = 0
  name = "test-app"
  charm {
    name     = "jameinel-ubuntu-lite"
    revision = 10
  }
  
  trust = true 
  expose{}
  constraints = "%s"
}
`, modelName, constraints)
	} else {
		return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = "test-app"
  charm {
    name     = "jameinel-ubuntu-lite"
	revision = 10
  }
  trust = true
  expose{}
  constraints = "%s"
  config = {
    juju-external-hostname="myhostname"
  }
}
`, modelName, constraints)
	}
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
    name     = "jameinel-ubuntu-lite"
    revision = 10
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
