// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	apispaces "github.com/juju/juju/api/client/spaces"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceApplication(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationBasic(modelName, appName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "jameinel-ubuntu-lite"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
					resource.TestCheckNoResourceAttr("juju_application.this", "storage"),
				),
			},
			{
				// Causes error on k8s that state is changing too quickly. Possibly run in a separate test.
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationWithFullySpecifiedModel(appName, expectedResourceOwner(), modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", expectedResourceOwner()+"/"+modelName),
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
					// Skipping this test due to a juju bug in 2.9:
					// Unable to create application, got error: charm
					// "state changing too quickly; try again soon"
					//
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

func TestAcc_ResourceApplication_Updates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "jameinel-ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		appName = "hello-kubecon"
	}

	resource.ParallelTest(t, resource.TestCase{
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
				// After the change for Update to call ReadApplicationWithRetryOnNotFound when
				// updating unit counts, charm revision/channel or storage this test has started to
				// fail with the known error: https://github.com/juju/terraform-provider-juju/issues/376
				// Expecting the error until this issue can be fixed.
				ExpectError: regexp.MustCompile("Provider produced inconsistent result after apply.*"),
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

func TestAcc_ResourceApplication_UpdateImportedSubordinate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	testAccPreCheck(t)

	modelName := acctest.RandomWithPrefix("tf-test-application")

	ctx := context.Background()

	_, err := TestClient.Models.CreateModel(juju.CreateModelInput{
		Name: modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = TestClient.Applications.CreateApplication(ctx, &juju.CreateApplicationInput{
		ApplicationName: "telegraf",
		ModelName:       modelName,
		CharmName:       "telegraf",
		CharmChannel:    "latest/stable",
		CharmRevision:   73,
		Units:           0,
	})
	if err != nil {
		t.Fatal(err)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:             testAccResourceApplicationSubordinate(modelName, 73),
				ImportState:        true,
				ImportStateId:      fmt.Sprintf("%s:telegraf", modelName),
				ImportStatePersist: true,
				ResourceName:       "juju_application.telegraf",
			},
			{
				Config: testAccResourceApplicationSubordinate(modelName, 75),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.telegraf", "charm.0.name", "telegraf"),
					resource.TestCheckResourceAttr("juju_application.telegraf", "charm.0.revision", "75"),
				),
			},
		},
	})
}

// TestAcc_ResourceApplication_UpdatesRevisionConfig will test the revision update that have new config parameters on
// the charm. The test will check that the config is updated and the revision is updated as well.
func TestAcc_ResourceApplication_UpdatesRevisionConfig(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "github-runner"
	configParamName := "runner-storage"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, appName, 88, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.name", appName),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.revision", "88"),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, appName, 96, configParamName, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.revision", "96"),
					resource.TestCheckResourceAttr("juju_application."+appName, "config."+configParamName, configParamName+"-value"),
				),
			},
		},
	})
}

func TestAcc_CharmUpdates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")

	resource.ParallelTest(t, resource.TestCase{
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

func TestAcc_ResourceRevisionUpdatesLXD(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-resource-revision-updates-lxd")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 21, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
			{
				// change resource revision to 3
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 21, "", "foo-file", "3"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "3"),
				),
			},
			{
				// change back to 4
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 21, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
		},
	})
}

func TestAcc_ResourceRevisionAddedToPlanLXD(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-resource-revision-updates-lxd")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 20, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.juju-qa-test", "resources"),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 21, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
		},
	})
}

func TestAcc_ResourceRevisionRemovedFromPlanLXD(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-resource-revision-updates-lxd")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// we specify the resource revision 4
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 20, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
			{
				// then remove the resource revision and update the charm revision
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "juju-qa-test", 21, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.juju-qa-test", "resources"),
				),
			},
		},
	})
}

func TestAcc_ResourceRevisionUpdatesMicrok8s(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-resource-revision-updates-microk8s")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "postgresql-k8s", 20, "", "postgresql-image", "152"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.postgresql-k8s", "resources.postgresql-image", "152"),
				),
			},
			{
				// change resource revision to 151
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "postgresql-k8s", 20, "", "postgresql-image", "151"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.postgresql-k8s", "resources.postgresql-image", "151"),
				),
			},
			{
				// change back to 152
				Config: testAccResourceApplicationWithRevisionAndConfig(modelName, "postgresql-k8s", 20, "", "postgresql-image", "152"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.postgresql-k8s", "resources.postgresql-image", "152"),
				),
			},
		},
	})
}

func TestAcc_CustomResourcesAddedToPlanMicrok8s(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Skipf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.0.3") < 0 {
		t.Skipf("%s is not set or is below 3.0.3", TestJujuAgentVersion)
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-resource-updates-microk8s")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// deploy charm without custom resource
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "1.0/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
			{
				// Add a custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/stable", "grafana-image", "gatici/grafana:10"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "gatici/grafana:10"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				// Add another custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/stable", "grafana-image", "gatici/grafana:9"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "gatici/grafana:9"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				// Add resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/stable", "grafana-image", "61"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "61"),
				),
			},
			{
				// Remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "1.0/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
		},
	})
}

func TestAcc_CustomResourceUpdatesMicrok8s(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Skipf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.0.3") < 0 {
		t.Skipf("%s is not set or is below 3.0.3", TestJujuAgentVersion)
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-resource-updates-microk8s")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Deploy charm with a custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/edge", "grafana-image", "gatici/grafana:9"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "gatici/grafana:9"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				// Keep charm channel and update resource to another custom image
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/edge", "grafana-image", "gatici/grafana:10"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "gatici/grafana:10"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				// Update charm channel and update resource to a revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/stable", "grafana-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "59"),
				),
			},
			{
				// Update charm channel and keep resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/beta", "grafana-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "59"),
				),
			},
			{
				// Keep charm channel and remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "1.0/beta"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
		},
	})
}

func TestAcc_CustomResourcesRemovedFromPlanMicrok8s(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Skipf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.0.3") < 0 {
		t.Skipf("%s is not set or is below 3.0.3", TestJujuAgentVersion)
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-resource-updates-microk8s")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Deploy charm with a custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/edge", "grafana-image", "gatici/grafana:9"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "gatici/grafana:9"),
				),
				ExpectNonEmptyPlan: true,
			},
			{
				// Keep charm channel and remove custom resource
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "1.0/edge"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
			{
				// Keep charm channel and add resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/edge", "grafana-image", "60"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "60"),
				),
			},
			{
				// Update charm channel and keep resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "1.0/stable", "grafana-image", "60"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.grafana-image", "60"),
				),
			},
			{
				// Update charm channel and remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "1.0/beta"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
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
	checkResourceAttr := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(resourceName, "model", modelName),
		resource.TestCheckResourceAttr(resourceName, "name", charmName),
		resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
	}
	if testingCloud == LXDCloudTesting {
		// Microk8s doesn't have machine, thus no placement
		checkResourceAttr = append(checkResourceAttr, resource.TestCheckResourceAttr(resourceName, "placement", "0"))
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationBasic_Minimal(modelName, charmName),
				Check: resource.ComposeTestCheckFunc(
					checkResourceAttr...),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceApplication_UpgradeProvider(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
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
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceApplicationBasic(modelName, appName),
				PlanOnly:                 true,
			},
		},
	})
}

func TestAcc_ResourceApplication_EndpointBindings(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-bindings")
	appName := "test-app"

	managementSpace, publicSpace, cleanUp := setupModelAndSpaces(t, modelName)
	defer cleanUp()

	constraints := "arch=amd64 spaces=" + managementSpace + "," + publicSpace
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// test creating a single application with default endpoint bound to management space, and ubuntu endpoint bound to public space
				Config: testAccResourceApplicationEndpointBindings(modelName, appName, constraints, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "ubuntu", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelName, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application." + appName,
			},
		},
	})
}

func TestAcc_ResourceApplication_UpdateEndpointBindings(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-bindings-update")
	appName := "test-app-update"

	managementSpace, publicSpace, cleanUp := setupModelAndSpaces(t, modelName)
	defer cleanUp()
	constraints := "arch=amd64 spaces=" + managementSpace + "," + publicSpace

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// test creating a single application with default endpoint bound to management space
				Config: testAccResourceApplicationEndpointBindings(modelName, appName, constraints, map[string]string{"": managementSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelName, appName, managementSpace, map[string]string{"": managementSpace}),
				),
			},
			{
				// updating the existing application's default endpoint to be bound to public space
				// this means all endpoints should be bound to public space (since no endpoint was on a different space)
				Config: testAccResourceApplicationEndpointBindings(modelName, appName, constraints, map[string]string{"": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelName, appName, publicSpace, map[string]string{"": publicSpace, "ubuntu": publicSpace, "another": publicSpace}),
				),
			},
			{
				// updating the existing application's default endpoint to be bound to management space, and specifying ubuntu endpoint to be bound to public space
				// this means all endpoints should be bound to public space, except for ubuntu which should be bound to public space
				Config: testAccResourceApplicationEndpointBindings(modelName, appName, constraints, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "ubuntu", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelName, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace, "another": managementSpace}),
				),
			},
			{
				// removing the endpoint bindings reverts to model's default space
				Config: testAccResourceApplicationEndpointBindings(modelName, appName, constraints, nil),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "0"),
					testCheckEndpointsAreSetToCorrectSpace(modelName, appName, "alpha", map[string]string{"": "alpha", "ubuntu": "alpha", "another": "alpha"}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application." + appName,
			},
		},
	})
}

func TestAcc_ResourceApplication_StorageLXD(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-storage")
	appName := "test-app-storage"

	storageConstraints := map[string]string{"label": "runner", "size": "2G"}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationStorageLXD(modelName, appName, storageConstraints),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage_directives.runner", "2G"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.label", "runner"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.count", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.size", "2G"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.pool", "lxd"),
				),
			},
		},
	})
}

func TestAcc_ResourceApplication_StorageK8s(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-storage")
	appName := "test-app-storage"

	storageConstraints := map[string]string{"label": "pgdata", "size": "2G"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationStorageK8s(modelName, appName, storageConstraints),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage_directives.pgdata", "2G"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.label", "pgdata"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.count", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.size", "2G"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.pool", "kubernetes"),
				),
			},
		},
	})
}

func TestAcc_ResourceApplicationWithDifferentModelOwner(t *testing.T) {
	appName := "test-app"
	modelName := "test"
	username := acctest.RandomWithPrefix("bob")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			// Create a model owned by a separate user.
			// We do this outside of Terraform because we don't currently support
			// creating a model owned by a different user than the one making the connection.
			cleanup := createModelWithNewUser(t, username, modelName)
			t.Cleanup(func() {
				cleanup()
			})
		},
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithExistingModel(appName, username, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", username+"/test"),
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "jameinel-ubuntu-lite"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				),
			},
		},
	})
}

func createModelWithNewUser(t *testing.T, username, modelName string) (cleanup func()) {
	testPassword := "test"
	userReq := internaljuju.CreateUserInput{
		Name:     username,
		Password: testPassword,
	}
	_, err := TestClient.Users.CreateUser(userReq)
	require.Nil(t, err)

	data := jujuProviderModelEnvVar()
	config := internaljuju.ControllerConfiguration{
		ControllerAddresses: strings.Split(data.ControllerAddrs.ValueString(), ","),
		Username:            username,
		Password:            testPassword,
		CACert:              data.CACert.ValueString(),
	}
	client, err := internaljuju.NewClient(context.Background(), config)
	require.Nil(t, err)

	// The next step has to be done by calling out to the Juju CLI until the provider supports these operations.
	cInfo := getControllerInfo(t)
	adminUser := data.UserName.ValueString()
	adminPassword := data.Password.ValueString()
	grantNewUserAccess(t, username, testPassword, adminUser, adminPassword, cInfo)

	createModelReq := internaljuju.CreateModelInput{
		Name:       modelName,
		Credential: cInfo.credentialName,
		CloudName:  cInfo.cloudName,
	}
	createModelResp, err := client.Models.CreateModel(createModelReq)
	require.Nil(t, err)

	grantModelReq := internaljuju.GrantModelInput{
		User:      data.UserName.ValueString(),
		Access:    "admin",
		ModelName: modelName,
	}
	err = client.Models.GrantModel(grantModelReq)
	require.Nil(t, err)

	return func() {
		destroyModelReq := internaljuju.DestroyModelInput{
			UUID: createModelResp.UUID,
		}
		err := TestClient.Models.DestroyModel(destroyModelReq)
		assert.Nil(t, err)
		destroyUserReq := internaljuju.DestroyUserInput{
			Name: username,
		}
		err = TestClient.Users.DestroyUser(destroyUserReq)
		assert.Nil(t, err)
	}
}

// grantNewUserAccess grants a new user with the ability to access the cloud being tested.
func grantNewUserAccess(t *testing.T, testUser, testPassword, adminUser, adminPassword string, cInfo testControllerInfo) {
	grantCloudCmd := exec.Command("juju", "grant-cloud", testUser, "add-model", cInfo.cloudName)
	_, err := grantCloudCmd.Output()
	require.Nil(t, err)

	logoutCmd := exec.Command("juju", "logout", "--force")
	_, err = logoutCmd.Output()
	require.Nil(t, err)

	loginCmd := exec.Command("juju", "login", "-u", testUser)
	passwordReader := strings.NewReader(testPassword + "\n")
	loginCmd.Stdin = passwordReader
	_, err = loginCmd.Output()
	require.Nil(t, err)

	addCredentialCmd := exec.Command("juju", "update-credential", cInfo.cloudName, cInfo.credentialName, "--controller", cInfo.controllerName)
	_, err = addCredentialCmd.Output()
	require.Nil(t, err)

	logoutCmd = exec.Command("juju", "logout", "--force")
	_, err = logoutCmd.Output()
	require.Nil(t, err)

	loginCmd = exec.Command("juju", "login", "-u", adminUser)
	passwordReader = strings.NewReader(adminPassword + "\n")
	loginCmd.Stdin = passwordReader
	_, err = loginCmd.Output()
	require.Nil(t, err)
}

type testControllerInfo struct {
	controllerName string
	cloudName      string
	credentialName string
}

func getControllerInfo(t *testing.T) testControllerInfo {
	whoamiCmd := exec.Command("juju", "whoami", "--format=json")
	out, err := whoamiCmd.Output()
	assert.Nil(t, err)
	whoamiInfo := make(map[string]any)
	require.Nil(t, json.Unmarshal(out, &whoamiInfo))
	controller, ok := whoamiInfo["controller"]
	require.True(t, ok)
	controllerName, ok := controller.(string)
	require.True(t, ok)

	cloudsCmd := exec.Command("juju", "clouds", "--controller", controllerName, "--format=json")
	out, err = cloudsCmd.Output()
	assert.Nil(t, err)
	cloudsInfo := make(map[string]any)
	require.Nil(t, json.Unmarshal(out, &cloudsInfo))
	require.Greater(t, len(cloudsInfo), 0)
	if len(cloudsInfo) > 1 {
		t.Logf("Multiple clouds found for test, choosing the first")
	}
	var cloudName string
	for key := range cloudsInfo {
		cloudName = key
		break
	}

	credentialsCmd := exec.Command("juju", "credentials", cloudName, "--client", "--format=json")
	out, err = credentialsCmd.Output()
	assert.Nil(t, err)
	credentials := make(map[string]any)
	require.Nil(t, json.Unmarshal(out, &credentials))
	credentialInfo, ok := credentials["client-credentials"].(map[string]any)
	assert.True(t, ok)
	require.Greater(t, len(credentialInfo), 0)
	if len(credentialInfo) > 1 {
		t.Logf("Multiple credentials found for test, choosing the first")
	}
	var credentialName string
	for key := range credentialInfo {
		credentialName = key
		break
	}

	return testControllerInfo{
		controllerName: controllerName,
		cloudName:      cloudName,
		credentialName: credentialName,
	}
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

func testAccResourceApplicationWithExistingModel(appName, username, modelName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationWithSharedModel",
		`	
resource "juju_application" "this" {
	name = "{{.AppName}}"
	model = "{{.Username}}/{{.ModelName}}"
	charm {
	name = "jameinel-ubuntu-lite"
	}
	trust = true
}
		`, internaltesting.TemplateData{
			"ModelName": modelName,
			"AppName":   appName,
			"Username":  username,
		})
}

func testAccResourceApplicationWithFullySpecifiedModel(appName, username, modelName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationWithFullySpecifiedModel",
		`
resource "juju_model" "this" {
	name = "{{.ModelName}}"
}
	
resource "juju_application" "this" {
	name = "{{.AppName}}"
	model = "{{.Username}}/${juju_model.this.name}"
	charm {
	name = "jameinel-ubuntu-lite"
	}
	trust = true
	expose{}
}
		`, internaltesting.TemplateData{
			"ModelName": modelName,
			"AppName":   appName,
			"Username":  username,
		})
}

func testAccResourceApplicationBasic(modelName, appName string) string {
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
		`, modelName, appName)
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
		`, modelName, appName)
	}
}

func testAccResourceApplicationWithRevisionAndConfig(modelName, appName string, revision int, configParamName string, resourceName string, resourceRevision string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationWithRevisionAndConfig",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  name  = "{{.AppName}}"
  model = juju_model.{{.ModelName}}.name

  charm {
    name     = "{{.AppName}}"
    revision = {{.Revision}}
    channel  = "latest/edge"
  }

  {{ if ne .ConfigParamName "" }}
  config = {
    {{.ConfigParamName}} = "{{.ConfigParamName}}-value"
  }
  {{ end }}

  {{ if ne .ResourceParamName "" }}
  resources = {
    {{.ResourceParamName}} = {{.ResourceParamRevision}}
  }
  {{ end }}

  units = 1
}
`, internaltesting.TemplateData{
			"ModelName":             modelName,
			"AppName":               appName,
			"Revision":              revision,
			"ConfigParamName":       configParamName,
			"ResourceParamName":     resourceName,
			"ResourceParamRevision": resourceRevision,
		})
}

func testAccResourceApplicationWithCustomResources(modelName, channel string, resourceName string, customResource string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = "test-app"
  charm {
    name     = "grafana-k8s"
	channel  = "%s"
  }
  trust = true
  expose{}
  resources = {
    "%s" = "%s"
  }
  config = {
    juju-external-hostname="myhostname"
  }
}
`, modelName, channel, resourceName, customResource)
}

func testAccResourceApplicationWithoutCustomResources(modelName, channel string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = "test-app"
  charm {
    name     = "grafana-k8s"
	channel  = "%s"
  }
  trust = true
  expose{}
  config = {
    juju-external-hostname="myhostname"
  }
}
`, modelName, channel)
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

func testAccResourceApplicationSubordinate(modelName string, subordinateRevision int) string {
	return fmt.Sprintf(`
resource "juju_application" "telegraf" {
  model = %q
  name = "telegraf"

  charm {
    name = "telegraf"
    revision = %d
  }

  units = 0
}
`, modelName, subordinateRevision)
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

func setupModelAndSpaces(t *testing.T, modelName string) (string, string, func()) {
	// All the space setup is needed until https://github.com/juju/terraform-provider-juju/issues/336 is implemented
	// called to have TestClient populated
	testAccPreCheck(t)
	model, err := TestClient.Models.CreateModel(internaljuju.CreateModelInput{
		Name: modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	conn, err := TestClient.Models.GetConnection(&modelName)
	if err != nil {
		t.Fatal(err)
	}
	cleanUp := func() {
		_ = TestClient.Models.DestroyModel(internaljuju.DestroyModelInput{UUID: model.UUID})
		_ = conn.Close()
	}

	managementBridgeCidr := os.Getenv("TEST_MANAGEMENT_BR")
	publicBridgeCidr := os.Getenv("TEST_PUBLIC_BR")
	if managementBridgeCidr == "" || publicBridgeCidr == "" {
		t.Skip("Management or Public bridge not set")
	}

	publicSpace := "public"
	managementSpace := "management"
	spaceAPIClient := apispaces.NewAPI(conn)
	err = spaceAPIClient.CreateSpace(managementSpace, []string{managementBridgeCidr}, true)
	if err != nil {
		t.Fatal(err)
	}
	err = spaceAPIClient.CreateSpace(publicSpace, []string{publicBridgeCidr}, true)
	if err != nil {
		t.Fatal(err)
	}

	return managementSpace, publicSpace, cleanUp
}

func testAccResourceApplicationEndpointBindings(modelName, appName, constraints string, endpointBindings map[string]string) string {
	var endpoints string
	for endpoint, space := range endpointBindings {
		if endpoint == "" {
			endpoints += fmt.Sprintf(`
		{
			"space"    = %q,
		},
		`, space)
		} else {
			endpoints += fmt.Sprintf(`
		{
			"endpoint" = %q,
			"space"    = %q,
		},
		`, endpoint, space)
		}
	}
	if len(endpoints) > 0 {
		endpoints = "[" + endpoints + "]"
	} else {
		endpoints = "null"
	}
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationEndpointBindings", `
data "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model       = data.juju_model.{{.ModelName}}.name
  name        = "{{.AppName}}"
  constraints = "{{.Constraints}}"
  charm {
    name     = "jameinel-ubuntu-lite"
    revision = 10
  }
  endpoint_bindings = {{.EndpointBindings}}
}
`, internaltesting.TemplateData{
		"ModelName":        modelName,
		"AppName":          appName,
		"Constraints":      constraints,
		"EndpointBindings": endpoints,
	})
}

func testAccResourceApplicationStorageLXD(modelName, appName string, storageConstraints map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model = juju_model.{{.ModelName}}.name
  name = "{{.AppName}}"
  charm {
    name = "github-runner"
    channel = "latest/stable"
    revision = 177
  }

  storage_directives = {
    {{.StorageConstraints.label}} = "{{.StorageConstraints.size}}"
  }

  units = 1
}
`, internaltesting.TemplateData{
		"ModelName":          modelName,
		"AppName":            appName,
		"StorageConstraints": storageConstraints,
	})
}

func testAccResourceApplicationStorageK8s(modelName, appName string, storageConstraints map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model = juju_model.{{.ModelName}}.name
  name = "{{.AppName}}"
  charm {
    name = "postgresql-k8s"
    channel = "14/stable"
  }

  storage_directives = {
    {{.StorageConstraints.label}} = "{{.StorageConstraints.size}}"
  }

  units = 1
}
`, internaltesting.TemplateData{
		"ModelName":          modelName,
		"AppName":            appName,
		"StorageConstraints": storageConstraints,
	})
}

func testCheckEndpointsAreSetToCorrectSpace(modelName, appName, defaultSpace string, configuredEndpoints map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := TestClient.Models.GetConnection(&modelName)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		applicationAPIClient := apiapplication.NewClient(conn)
		clientAPIClient := apiclient.NewClient(conn, TestClient.Applications.JujuLogger())

		apps, err := applicationAPIClient.ApplicationsInfo([]names.ApplicationTag{names.NewApplicationTag(appName)})
		if err != nil {
			return err
		}
		if len(apps) > 1 {
			return fmt.Errorf("more than one result for application: %s", appName)
		}
		if len(apps) < 1 {
			return fmt.Errorf("no results for application: %s", appName)
		}
		if apps[0].Error != nil {
			return apps[0].Error
		}

		appInfo := apps[0].Result
		appInfoBindings := appInfo.EndpointBindings

		var appStatus params.ApplicationStatus
		var exists bool
		// Block on the application being active
		// This is needed to make sure the units have access
		// to ip addresses part of the spaces
		for i := 0; i < 50; i++ {
			status, err := clientAPIClient.Status(&apiclient.StatusArgs{
				Patterns: []string{appName},
			})
			if err != nil {
				return err
			}
			appStatus, exists = status.Applications[appName]
			if exists && appStatus.Status.Status == "active" {
				break
			}
			if exists && appStatus.Status.Status == "error" {
				return fmt.Errorf("application %s has error status", appName)
			}
			time.Sleep(10 * time.Second)
		}
		if !exists {
			return fmt.Errorf("no status returned for application: %s", appName)
		}
		if appStatus.Status.Status != "active" {
			return fmt.Errorf("application %s is not active, status: %s", appName, appStatus.Status.Status)
		}
		for endpoint, space := range appInfoBindings {
			if ep, ok := configuredEndpoints[endpoint]; ok {
				if ep != space {
					return fmt.Errorf("endpoint %q is bound to %q, expected %q", endpoint, space, ep)
				}
			} else {
				if space != defaultSpace {
					return fmt.Errorf("endpoint %q is bound to %q, expected %q", endpoint, space, defaultSpace)
				}
			}
		}
		return nil
	}
}
