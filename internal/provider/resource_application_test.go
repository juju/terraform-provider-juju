// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
					resource.TestCheckNoResourceAttr("juju_application.this", "storage"),
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

func TestAcc_ResourceApplication_ConstraintsNormalization(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationConstraints(modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
					resource.TestCheckResourceAttr("juju_application.this", "machines.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "machines.0", "0"),
				),
			},
			{
				Config: testAccResourceApplicationConstraints(modelName, "mem=4096M cores=1 arch=amd64"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "mem=4096M cores=1 arch=amd64"),
					// assert that machines have not changed due to changed order in constraints
					resource.TestCheckResourceAttr("juju_application.this", "machines.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "machines.0", "0"),
				),
			},
		},
	})
}

func TestAcc_ResourceApplicationScaleUp(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application-scale-up")
	appName := "test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccResourceApplicationScaleUp(modelName, appName, "1"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
			),
		}, {
			Config: testAccResourceApplicationScaleUp(modelName, appName, "2"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			),
		}, {
			Config: testAccResourceApplicationScaleUp(modelName, appName, "1"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
			),
		}},
	})
}

func TestAcc_ResourceApplication_Updates(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		appName = "coredns"
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
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "4"),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != MicroK8sTesting, nil
				},
				Config: testAccResourceApplicationUpdates(modelName, 2, true, "machinename"),
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "165"),
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

func TestAcc_CharmUpdateBase(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmbaseupdates")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationUpdateBaseCharm(modelName, "ubuntu@22.04"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.base", "ubuntu@22.04"),
				),
			},
			{
				// move to base ubuntu 20.04
				Config: testAccApplicationUpdateBaseCharm(modelName, "ubuntu@20.04"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.base", "ubuntu@20.04"),
				),
			},
			{
				// move back to ubuntu 22.04
				Config: testAccApplicationUpdateBaseCharm(modelName, "ubuntu@22.04"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.base", "ubuntu@22.04"),
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
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
			// In the next step we verify the plan has no changes. First waiting 30 seconds
			// to avoid a race condition in Juju where updating the resource revision too
			// quickly means that the change doesn't take immediate effect.
			{
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
				PreConfig: func() {
					time.Sleep(30 * time.Second)
				},
			},
			{
				// Add a custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/stable", "coredns-image", "ghcr.io/canonical/test:6a873fb35b0170dfe49ed27ba8ee6feb8e475131"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "ghcr.io/canonical/test:6a873fb35b0170dfe49ed27ba8ee6feb8e475131"),
				),
			},
			{
				// Add another custom resource
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/stable", "coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				),
			},
			{
				// Add resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/stable", "coredns-image", "69"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "69"),
				),
			},
			{
				// Remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
				// We need to wait 30 seconds to let the charm's agent to settle. Otherwise,
				// after we try to destroy the application the agent can go into `lost` state,
				// making the test waits on application destroy until the timeout is reached.
				// This is not an issue because if we reach the timeout we don't error out,
				// but it slows down the test suite.
				PreConfig: func() {
					time.Sleep(30 * time.Second)
				},
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
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/edge", "coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				),
			},
			{
				// Keep charm channel and update resource to another custom image
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/edge", "coredns-image", "ghcr.io/canonical/test:6a873fb35b0170dfe49ed27ba8ee6feb8e475131"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "ghcr.io/canonical/test:6a873fb35b0170dfe49ed27ba8ee6feb8e475131"),
				),
			},
			{
				// Update charm channel and update resource to a revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/stable", "coredns-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "59"),
				),
			},
			{
				// Update charm channel and keep resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/beta", "coredns-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "59"),
				),
			},
			{
				// Keep charm channel and remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/beta"),
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
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/edge", "coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"),
				),
			},
			{
				// Keep charm channel and remove custom resource
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/edge"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
			{
				// Keep charm channel and add resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/edge", "coredns-image", "60"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "60"),
				),
			},
			{
				// Update charm channel and keep resource revision
				Config: testAccResourceApplicationWithCustomResources(modelName, "latest/stable", "coredns-image", "60"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "resources.coredns-image", "60"),
				),
			},
			{
				// Update charm channel and remove resource revision
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/beta"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
			},
			{
				// Add a dummy final step to allow the app to settle before destroying the environment.
				PreConfig: func() {
					fmt.Println("Final wait before destroying the model")
					time.Sleep(30 * time.Second)
				},
				RefreshState: true,
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

func TestAcc_ResourceApplication_Subordinate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-subordinate")

	ntpName := "juju_application.ntp"

	checkResourceAttrSubordinate := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(ntpName, "model", modelName),
		resource.TestCheckResourceAttr(ntpName, "units", "1"),
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccResourceApplicationBasic_ntp_Subordinates(modelName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrSubordinate...),
		}, {
			ImportStateVerify: true,
			ImportState:       true,
			ResourceName:      ntpName,
		}},
	})
}

func testAccResourceApplicationBasic_ntp_Subordinates(modelName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_application" "ntp" {
			model = juju_model.model.name
			name = "ntp"
			charm {
				name = "ntp"
				base = "ubuntu@22.04"
			}
		}
		`, modelName)
}

func TestAcc_ResourceApplication_MachinesWithSubordinates(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-machines-with-subordinates")

	charmName := "juju-qa-test"

	resourceName := "juju_application.testapp"
	ntpName := "juju_application.ntp"
	numberOfMachines := 10

	checkResourceAttrMachines := func(numberOfMachines int) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(resourceName, "model", modelName),
			resource.TestCheckResourceAttr(resourceName, "name", charmName),
			resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
			resource.TestCheckResourceAttr(resourceName, "units", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(resourceName, "machines.#", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(ntpName, "model", modelName),
			resource.TestCheckResourceAttr("juju_integration.testapp_ntp", "application.#", "2"),
		}
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			ConfigVariables: config.Variables{
				"machines": config.IntegerVariable(numberOfMachines),
			},
			Config: testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(numberOfMachines)...),
		}, {
			ConfigVariables: config.Variables{
				"machines": config.IntegerVariable(numberOfMachines - 1),
			},
			Config: testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(numberOfMachines - 1)...),
		}, {
			ConfigVariables: config.Variables{
				"machines": config.IntegerVariable(2),
			},
			Config: testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(2)...),
		}, {
			ImportStateVerify: true,
			ImportState:       true,
			ResourceName:      resourceName,
		}},
	})
}

func TestAcc_ResourceApplication_SwitchMachinestoUnits(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-switch-machines-to-units")

	charmName := "juju-qa-test"

	resourceName := "juju_application.testapp"
	ntpName := "juju_application.ntp"
	numberOfMachines := 3

	checkResourceAttrMachines := func(numberOfMachines int) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(resourceName, "model", modelName),
			resource.TestCheckResourceAttr(resourceName, "name", charmName),
			resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
			resource.TestCheckResourceAttr(resourceName, "units", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(resourceName, "machines.#", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(ntpName, "model", modelName),
			resource.TestCheckResourceAttr("juju_integration.testapp_ntp", "application.#", "2"),
		}
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			ConfigVariables: config.Variables{
				"machines": config.IntegerVariable(numberOfMachines),
			},
			Config: testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(numberOfMachines)...),
		}, {
			ConfigVariables: config.Variables{
				"machines": config.IntegerVariable(numberOfMachines),
			},
			Config: testAccResourceApplicationBasic_UnitsWithSubordinates(modelName, charmName, fmt.Sprintf("%d", numberOfMachines)),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(numberOfMachines)...),
		}, {
			ImportStateVerify: true,
			ImportState:       true,
			ResourceName:      resourceName,
		}},
	})
}

func testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "all_machines" {
			count = var.machines
  			model = juju_model.model.name
			base = "ubuntu@22.04"
			name = "machine_${count.index}"

			# The following lifecycle directive instructs Terraform to update 
			# any dependent resources before destroying the machine - in the 
			# case of applications this means that application units get 
			# removed from units before Terraform attempts to destroy the 
			# machine.
			lifecycle {
				create_before_destroy = true
			}
		}

		resource "juju_application" "testapp" {
		  name = "juju-qa-test"
		  model = juju_model.model.name


		  machines = toset( juju_machine.all_machines[*].machine_id )

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}

		resource "juju_application" "ntp" {
			model = juju_model.model.name
			name = "ntp"

			charm {
				name = "ntp"
				base = "ubuntu@22.04"
			}
		}

		resource "juju_integration" "testapp_ntp" {
			model = juju_model.model.name

			application {
				name = juju_application.testapp.name
				endpoint = "juju-info"
			}

			application {
				name = juju_application.ntp.name
			}
		}

		variable "machines" {
			description = "Number of machines to deploy."
			type = number
			default = 1
		}
		`, modelName, charmName)
}

func testAccResourceApplicationBasic_UnitsWithSubordinates(modelName, charmName, numberOfUnits string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "all_machines" {
			count = var.machines
  			model = juju_model.model.name
			base = "ubuntu@22.04"
			name = "machine_${count.index}"
		}

		resource "juju_application" "testapp" {
		  name = "juju-qa-test"
		  model = juju_model.model.name


		  units = %q

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}

		resource "juju_application" "ntp" {
			model = juju_model.model.name
			name = "ntp"

			charm {
				name = "ntp"
				base = "ubuntu@22.04"
			}
		}

		resource "juju_integration" "testapp_ntp" {
			model = juju_model.model.name

			application {
				name = juju_application.testapp.name
				endpoint = "juju-info"
			}

			application {
				name = juju_application.ntp.name
			}
		}

		variable "machines" {
			description = "Number of machines to deploy."
			type = number
			default = 1
		}
		`, modelName, numberOfUnits, charmName)
}

func TestAcc_ResourceApplication_Machines(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-placement-machines")

	charmName := "juju-qa-test"

	resourceName := "juju_application.testapp"
	checkResourceAttrPlacement := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(resourceName, "model", modelName),
		resource.TestCheckResourceAttr(resourceName, "name", charmName),
		resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
		resource.TestCheckResourceAttr(resourceName, "units", "1"),
	}
	checkResourceAttrMachines := []resource.TestCheckFunc{
		resource.TestCheckResourceAttr(resourceName, "model", modelName),
		resource.TestCheckResourceAttr(resourceName, "name", charmName),
		resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
		resource.TestCheckResourceAttr(resourceName, "units", "1"),
		resource.TestCheckResourceAttr(resourceName, "machines.0", "0"),
	}
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationBasic_Placement(modelName, charmName),
				Check: resource.ComposeTestCheckFunc(
					checkResourceAttrPlacement...),
			},
			{
				Config: testAccResourceApplicationBasic_Machines(modelName, charmName),
				Check: resource.ComposeTestCheckFunc(
					checkResourceAttrMachines...),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceApplicationBasic_Placement(modelName, charmName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "machine" {
		  name = "test machine"
		  model = juju_model.model.name
		  base = "ubuntu@22.04"
		}

		resource "juju_application" "testapp" {
		  model = juju_model.model.name

		  units = 1
		  placement =  "${join(",", [juju_machine.machine.machine_id])}"

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}
		`, modelName, charmName)
}

func testAccResourceApplicationBasic_Machines(modelName, charmName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "machine" {
		  name = "test machine"
		  model = juju_model.model.name
		  base = "ubuntu@22.04"
		}

		resource "juju_application" "testapp" {
		  model = juju_model.model.name

		  machines = [juju_machine.machine.machine_id]

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}
		`, modelName, charmName)
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
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "expose.#", "1"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceApplicationBasic(modelName, appName),
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

	storageConstraints := map[string]string{"label": "pgdata", "size": "1M"}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationStorageLXD(modelName, appName, storageConstraints),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage_directives.pgdata", "1M"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.label", "pgdata"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.count", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.size", "1M"),
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

	storageConstraints := map[string]string{"label": "pgdata", "size": "1M"}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationStorageK8s(modelName, appName, storageConstraints),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage_directives.pgdata", "1M"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.label", "pgdata"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.count", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.size", "1M"),
					resource.TestCheckResourceAttr("juju_application."+appName, "storage.0.pool", "kubernetes"),
				),
			},
		},
	})
}

func TestAcc_ResourceApplication_UnsetConfigUsingNull(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application-unset-config")
	appName := "test-app"
	resourceName := "juju_application.test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccApplicationConfigNull(modelName, appName, "\"xxx\"", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", appName),
					resource.TestCheckResourceAttr(resourceName, "config.token", "xxx"),
				),
			},
			{
				Config: testAccApplicationConfigNull(modelName, appName, "null", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", appName),
					resource.TestCheckNoResourceAttr(resourceName, "config.token"),
				),
			},
			{
				Config: testAccApplicationConfigNull(modelName, appName, "null", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", appName),
					resource.TestCheckNoResourceAttr(resourceName, "config.token"),
				),
			},
			{
				Config: testAccApplicationConfigNull(modelName, appName, "\"ABC\"", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", appName),
					resource.TestCheckResourceAttr(resourceName, "config.token", "ABC"),
				),
			},
			{
				Config: testAccApplicationConfigNull(modelName, appName, "\"\"", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", appName),
					resource.TestCheckResourceAttr(resourceName, "config.token", ""),
				),
			},
		},
	})
}

func testAccApplicationConfigNull(modelName, appName, configValue string, includeConfig bool) string {
	return internaltesting.GetStringFromTemplateWithData("testAccApplicationConfigNull", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model = juju_model.{{.ModelName}}.name
  name  = "{{.AppName}}"
  charm {
	name = "juju-qa-dummy-source"
  }
  {{ if .IncludeConfig }}
  config = {
	token = {{.ConfigValue}}
  }
  {{ end }}
}
`, internaltesting.TemplateData{
		"ModelName":     modelName,
		"AppName":       appName,
		"ConfigValue":   configValue,
		"IncludeConfig": includeConfig,
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
			name = "ubuntu-lite"
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
			name = "ubuntu-lite"
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

func testAccResourceApplicationScaleUp(modelName, appName, numberOfUnits string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = %q
		  charm {
			name = "ubuntu-lite"
		  }
		  trust = true
		  expose{}
		  units = %q
		}
		`, modelName, appName, numberOfUnits)
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
			name = "ubuntu-lite"
		  }
		  trust = true
		  expose{}
		  units = %q
		  config = {
			juju-external-hostname="myhostname"
		  }
		}
		`, modelName, appName, numberOfUnits)
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
    name     = "coredns"
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
    name     = "coredns"
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
			name     = "ubuntu-lite"
			revision = 4
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
			name     = "coredns"
			revision = 165
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
			name     = "coredns"
			channel = %q
		  }
		}
		`, modelName, channel)
	}
}

func testAccApplicationUpdateBaseCharm(modelName string, base string) string {
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
			base = %q
		  }
		}
		`, modelName, base)
	} else {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model = juju_model.this.name
		  name = "test-app"
		  charm {
			name     = "coredns"
			channel = "1.25/stable"
			base = %q
		  }
		}
		`, modelName, base)
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
  units = 1
  name = "test-app"
  charm {
    name     = "ubuntu-lite"
    revision = 2
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
    name     = "ubuntu-lite"
	revision = 2
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
    name     = "ubuntu-lite"
    revision = 2
  }
  trust = true
  expose{}
  constraints = "%s"
}

resource "juju_application" "subordinate" {
  model = juju_model.this.name
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
    name     = "ubuntu-lite"
    revision = 2
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
    name = "postgresql"
	channel = "14/stable"
	revision = 553
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
	revision = 300
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

func TestAcc_ResourceApplication_ParallelDeploy(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application-parallel-deploy")
	appName1 := "test-app-a"
	appName2 := "test-app-b"

	var charm, channel string
	switch testingCloud {
	case MicroK8sTesting:
		charm = "juju-qa-test"
		channel = "latest/stable"
	case LXDCloudTesting:
		charm = "juju-qa-test"
		channel = "latest/stable"
	default:
		t.Fatalf("unknown test cloud")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationParallelDeploy(modelName, appName1, appName2, charm, channel),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName1, "model", modelName),
					resource.TestCheckResourceAttr("juju_application."+appName2, "model", modelName),
				),
			},
		},
	})
}

func testAccResourceApplicationParallelDeploy(modelName, appName1, appName2, charm, channel string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName1}}" {
  model = juju_model.{{.ModelName}}.name
  name = "{{.AppName1}}"
  charm {
    name = "{{.CharmName}}"
    channel = "{{.CharmChannel}}"
  }
  units = 1
}

resource "juju_application" "{{.AppName2}}" {
  model = juju_model.{{.ModelName}}.name
  name = "{{.AppName2}}"
  charm {
    name = "{{.CharmName}}"
    channel = "{{.CharmChannel}}"
  }
  units = 1
}
`, internaltesting.TemplateData{
		"ModelName":    modelName,
		"AppName1":     appName1,
		"AppName2":     appName2,
		"CharmName":    charm,
		"CharmChannel": channel,
	})
}

func TestAcc_ResourceApplication_CustomOCIForResource(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with MIcroK8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-oci-resource")
	charm := "coredns"
	resourceName := "coredns-image"
	ociImage := "ghcr.io/canonical/test:6a873fb35b0170dfe49ed27ba8ee6feb8e475131"
	ociImage2 := "ghcr.io/canonical/test:ab0b183f22db2959e0350f54d92f9ed3583c4167"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOciImage(modelName, charm, resourceName, ociImage),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.test", "resources."+resourceName, ociImage),
				),
			},
			{
				Config: testAccResourceOciImage(modelName, charm, resourceName, ociImage2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.test", "resources."+resourceName, ociImage2),
				),
			},
		},
	})
}

func testAccResourceOciImage(modelName, charm, resourceName, ociImage string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "test" {
  model = juju_model.{{.ModelName}}.name
  name = "test"
  charm {
	name = "{{.CharmName}}"
	revision = 18
  }
  resources = {
	"{{.ResourceName}}" = "{{.OciImage}}"
  }
  units = 1
}
`, internaltesting.TemplateData{
		"ModelName":    modelName,
		"CharmName":    charm,
		"ResourceName": resourceName,
		"OciImage":     ociImage,
	})
}

func TestAcc_ResourceApplication_UpdateEmptyConfig(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// create application with one config value present, and default trust = false
			{
				Config: testAccResourceApplicationUpdateConfig(modelName, appName, false, map[string]string{"config-file": "xxx"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "conserver"),
					resource.TestCheckResourceAttr("juju_application.this", "trust", "false"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "config.config-file", "xxx"),
				),
			},
			// reset first config values, add a different one
			{
				Config: testAccResourceApplicationUpdateConfig(modelName, appName, false, map[string]string{"passwd-file": "yyy"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "trust", "false"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "config.passwd-file", "yyy"),
				),
			},
			// reset all values, pass empty map
			{
				Config: testAccResourceApplicationUpdateConfig(modelName, appName, false, map[string]string{}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "trust", "false"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "0"),
				),
			},
			// set config value to non-empty, to prepare for next step
			{
				Config: testAccResourceApplicationUpdateConfig(modelName, appName, false, map[string]string{"config-file": "xxx", "passwd-file": "yyy"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "trust", "false"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "2"),
					resource.TestCheckResourceAttr("juju_application.this", "config.config-file", "xxx"),
					resource.TestCheckResourceAttr("juju_application.this", "config.passwd-file", "yyy"),
				),
			},
			// set trust to true and remove a config entry in a single update
			{
				Config: testAccResourceApplicationUpdateConfig(modelName, appName, true, map[string]string{"config-file": "xxx"}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "config.config-file", "xxx"),
				),
			},
			// test removal of config map altogether, and not just the entries
			{
				Config: testAccResourceApplicationRemoveConfig(modelName, appName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "trust", "false"),
					resource.TestCheckResourceAttr("juju_application.this", "config.%", "0"),
				),
			},
		},
	})
}

func testAccResourceApplicationUpdateConfig(modelName, appName string, trust bool, configMap map[string]string) string {
	configStr := ""
	for key, value := range configMap {
		configStr += fmt.Sprintf("%s = \"%s\"\n", key, value)
	}
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = %q
  charm {
	name = "conserver"
  }
  trust = %t
  config = {
	%s
  }
  units = 1
}
		`, modelName, appName, trust, configStr)
}

func testAccResourceApplicationRemoveConfig(modelName, appName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = %q
  charm {
	name = "conserver"
  }
  trust = false
  units = 1
}
		`, modelName, appName)
}

func TestAcc_ResourceApplication_WithConstraints(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"
	constraints := "mem=256"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithConstraints(modelName, appName, constraints),
			},
		},
	})
}

func testAccResourceApplicationWithConstraints(modelName, appName, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model = juju_model.this.name
  name = %q
  charm {
	name = "ubuntu-lite"
  }
  units = 1
  constraints = %q
}
		`, modelName, appName, constraints)
}
