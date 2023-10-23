// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceApplication_Edge(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"
	appInvalidName := "test_app"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceApplicationBasic(modelName, appInvalidName),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Unable to create application, got error: invalid application name %q,\nunexpected character _", appInvalidName)),
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
				),
			},
			{
				// cores constraint is not valid in K8s
				SkipFunc: func() (bool, error) {
					// Failing with terraform 1.4.x and 1.3.x
					// Unable to create application, got error: charm
					//" ch:amd64/focal/jameinel-ubuntu-lite-10" not found (not found)
					//
					// Related to:
					// https://github.com/juju/terraform-provider-juju/issues/272
					// There is a timing window with destroying an application
					// before a new one is created when RequiresReplace is used in the
					// resource schema.
					return true, nil
					//return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationConstraints(modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
				),
			},
			{
				// specific constraints for k8s
				SkipFunc: func() (bool, error) {
					// Skipping this test due to a juju bug in 2.9:
					// Unable to create application, got error: charm
					// "state changing too quickly; try again soon"
					//
					// Also related to:
					// https://github.com/juju/terraform-provider-juju/issues/272
					// There is a timing window with destroying an application
					// before a new one is created when RequiresReplace is used in the
					// resource schema.
					return true, nil
					//return testingCloud != MicroK8sTesting, nil
				},
				Config: testAccResourceApplicationConstraints(modelName, "arch=amd64 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 mem=4096M"),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					// Failing with terraform 1.4.x and 1.3.x
					// Unable to create application, got error: charm
					//" ch:amd64/focal/jameinel-ubuntu-lite-10" not found (not found)
					//
					// Related to:
					// https://github.com/juju/terraform-provider-juju/issues/272
					// There is a timing window with destroying an application
					// before a new one is created when RequiresReplace is used in the
					// resource schema.
					return true, nil
					//return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationConstraintsSubordinate(modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
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

func TestAcc_ResourceApplication_Updates_Edge(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "jameinel-ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		appName = "hello-kubecon"
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
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

func TestAcc_CharmUpdates_Edge(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
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

func TestAcc_ResourceApplication_Minimal(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	var charmName string
	if testingCloud == LXDCloudTesting {
		charmName = "juju-qa-test"
	} else {
		charmName = "hello-juju"
	}
	resourceName := "juju_application.testapp"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationBasic_Minimal(modelName, charmName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckResourceAttr(resourceName, "name", charmName),
					resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceApplication_Stable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"
	appInvalidName := "test_app"

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
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
				),
			},
			{
				SkipFunc: func() (bool, error) {
					// cores constraint is not valid in K8s
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationConstraints(modelName, "arch=amd64 cores=1 mem=4096M"),
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
				Config: testAccResourceApplicationConstraints(modelName, "arch=amd64 mem=4096M"),
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
				Config: testAccResourceApplicationConstraintsSubordinate(modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
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

func TestAcc_ResourceApplication_Updates_Stable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "jameinel-ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		appName = "hello-kubecon"
	}
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
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
				SkipFunc: func() (bool, error) {
					// This test is broken in a released version of the juju
					// provider, 0.8.0, but not in the version under development
					// === RUN   TestAcc_ResourceApplication_Updates_Stable
					//    resource_application_test.go:485: Skipping step 2/7 due to SkipFunc
					//    resource_application_test.go:485: Skipping step 3/7 due to SkipFunc
					//    resource_application_test.go:485: Step 7/7 error running import: ImportStateVerify attributes not equivalent. Difference is shown below. The - symbol indicates attributes missing after import.
					//
					//        map[string]string{
					//        	"placement": "",
					//        	"placement": ",",
					//        }
					//--- FAIL: TestAcc_ResourceApplication_Updates_Stable (25.87s)
					return testingCloud != MicroK8sTesting && TestProviderStableVersion == "0.8.0", nil
				},
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.this",
			},
		},
	})
}

func TestAcc_CharmUpdates_Stable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
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

func testAccResourceApplicationBasic_Minimal(modelName, charmName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "testmodel" {
		  name = %q
		}
		
		resource "juju_application" "testapp" {
		  model = juju_model.testmodel.name
		  charm {
			name = %q
		  }
		}
		`, modelName, charmName)
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
func testAccResourceApplicationConstraints(modelName string, constraints string) string {
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

func testAccResourceApplicationConstraintsSubordinate(modelName string, constraints string) string {
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
