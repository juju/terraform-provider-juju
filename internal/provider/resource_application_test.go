// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/api/client/resources"
	apispaces "github.com/juju/juju/api/client/spaces"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"

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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 mem=4096M"),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceApplicationConstraintsSubordinate(modelName, "arch=amd64 cores=1 mem=4096M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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

func testAccResourceApplicationExpose(modelName, endpoints string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "app" {
  name     = "app"
  model_uuid    = juju_model.this.uuid
  expose {
    endpoints = %q
  }
  charm {
    name     = "apache2"
    channel  = "latest/stable"
    revision = 64
    base     = "ubuntu@22.04"
  }
}
		`, modelName, endpoints)
}

func TestAcc_ResourceApplication_Expose(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-application-expose")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationExpose(modelName, "website"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.app", "model_uuid"),
					resource.TestCheckResourceAttr("juju_application.app", "name", "app"),
					resource.TestCheckResourceAttr("juju_application.app", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.app", "charm.0.name", "apache2"),
					resource.TestCheckResourceAttr("juju_application.app", "expose.#", "1"),
					resource.TestCheckResourceAttr("juju_application.app", "expose.0.endpoints", "website"),
				),
			},
			{
				Config: testAccResourceApplicationExpose(modelName, "apache-website,website"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.app", "model_uuid"),
					resource.TestCheckResourceAttr("juju_application.app", "name", "app"),
					resource.TestCheckResourceAttr("juju_application.app", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.app", "charm.0.name", "apache2"),
					resource.TestCheckResourceAttr("juju_application.app", "expose.#", "1"),
					resource.TestCheckResourceAttr("juju_application.app", "expose.0.endpoints", "apache-website,website"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_application.app",
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_application.this", "constraints", "arch=amd64 cores=1 mem=4096M"),
					resource.TestCheckResourceAttr("juju_application.this", "machines.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "machines.0", "0"),
				),
			},
			{
				Config: testAccResourceApplicationConstraints(modelName, "mem=4096M cores=1 arch=amd64"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
			),
		}, {
			Config: testAccResourceApplicationScaleUp(modelName, appName, "2"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
			),
		}, {
			Config: testAccResourceApplicationScaleUp(modelName, appName, "1"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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

// TestAcc_ResourceApplication_RefreshCharmUpdatesResources
// verifies that when an application does not specify resource revisions,
// an update to the charm revision will update the resource revisions to
// the latest available ones.
func TestAcc_ResourceApplication_RefreshCharmUpdatesResources(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationRefreshCharmUpdatesResources(modelName, 165),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
					// Use a check to grab the model UUID and update the application's resource
					// to a specific revision.
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["juju_model.this"]
						if !ok {
							return fmt.Errorf("not found: juju_model.this")
						}
						modelUUID := rs.Primary.Attributes["uuid"]
						// Use application client to update the application's resource image
						input := internaljuju.UpdateApplicationInput{
							ModelUUID: modelUUID,
							AppName:   "test-app",
							Resources: map[string]internaljuju.CharmResource{
								"coredns-image": {
									RevisionNumber: "60",
								},
							},
						}
						err := TestClient.Applications.UpdateApplication(&input)
						if err != nil {
							return err
						}
						// Read the application to verify the resource revision is set to 60
						// and the charm revision is 165.
						readInput := internaljuju.ReadApplicationInput{
							ModelUUID: modelUUID,
							AppName:   "test-app",
						}
						readRes, err := TestClient.Applications.ReadApplication(&readInput)
						if err != nil {
							return err
						}
						if readRes.Revision != 165 {
							return fmt.Errorf("expected charm revision to be 165, got %d", readRes.Revision)
						}
						coreDNSImage, ok := readRes.Resources["coredns-image"]
						if !ok {
							return fmt.Errorf("coredns-image resource not found")
						}
						if coreDNSImage != "60" {
							return fmt.Errorf("expected coredns-image resource revision to be 60, got %s", coreDNSImage)
						}
						return nil
					},
				),
			},
			{
				Config: testAccResourceApplicationRefreshCharmUpdatesResources(modelName, 166),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "166"),
					// Use a check to grab the model UUID and verify that the application's
					// resource revision has been updated to the latest (greater than 60).
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["juju_model.this"]
						if !ok {
							return fmt.Errorf("not found: juju_model.this")
						}
						modelUUID := rs.Primary.Attributes["uuid"]
						input := internaljuju.ReadApplicationInput{
							ModelUUID: modelUUID,
							AppName:   "test-app",
						}
						res, err := TestClient.Applications.ReadApplication(&input)
						if err != nil {
							return err
						}
						coreDNSImage, ok := res.Resources["coredns-image"]
						if !ok {
							return fmt.Errorf("coredns-image resource not found")
						}
						imgRevision, err := strconv.Atoi(coreDNSImage)
						if err != nil {
							return err
						}
						if imgRevision <= 60 {
							return fmt.Errorf("expected coredns-image resource revision to be greater than 60, got %d", imgRevision)
						}
						return nil
					},
				),
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

	resp, err := TestClient.Models.CreateModel(internaljuju.CreateModelInput{
		Name: modelName,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, err = TestClient.Applications.CreateApplication(ctx, &internaljuju.CreateApplicationInput{
		ApplicationName: "telegraf",
		ModelUUID:       resp.UUID,
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
				Config:             testAccResourceApplicationSubordinate(resp.UUID, 73),
				ImportState:        true,
				ImportStateId:      fmt.Sprintf("%s:telegraf", resp.UUID),
				ImportStatePersist: true,
				ResourceName:       "juju_application.telegraf",
			},
			{
				Config: testAccResourceApplicationSubordinate(resp.UUID, 75),
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
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
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

func TestAcc_CharmUpdatesWithRevision(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationUpdatesCharmWithRevision(modelName, "2.0/stable", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "2.0/stable"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "22"),
				),
			},
			{
				Config: testAccResourceApplicationUpdatesCharmWithRevision(modelName, "2.0/edge", "23"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "2.0/edge"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "23"),
				),
			},
			{
				Config: testAccResourceApplicationUpdatesCharmWithRevision(modelName, "2.0/stable", "22"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "2.0/stable"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "22"),
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
		},
	})
}

func TestAcc_CustomResourcesFromPrivateRegistry(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-resource-file")
	appName := "test-app"
	appResourceFullName := "juju_application." + appName
	// - Remove the custom resource.

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// A custom resource from a private registry.
				Config: testAccResourceApplicationFromPrivateRegistry(modelName, appName, "user", "pass", "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApplicationResource(appResourceFullName, charmResourceChecks{
						fingerprint: "1b94afe549b44328f2350ae24633b31265a01e466cf0469faa798acb9c637bea30c3c711f25937795eff34d2f920e074",
						origin:      "upload",
						revision:    "0",
					}),
				),
			},
			{
				// A custom resource and changed registry credentials.
				Config: testAccResourceApplicationFromPrivateRegistry(modelName, appName, "user2", "pass2", "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(appResourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApplicationResource(appResourceFullName, charmResourceChecks{
						fingerprint: "953991156cf1e0a601f52b2b2b16c7042ad13bf765655c024f384385306404b7eb30bf72bdfcfda3c570b076b3aa96dc",
						origin:      "upload",
						revision:    "0",
					}),
				),
			},
			{
				// A custom resource and removed registry credentials.
				Config: testAccResourceApplicationFromPrivateRegistry(modelName, appName, "", "", "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(appResourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApplicationResource(appResourceFullName, charmResourceChecks{
						fingerprint: "591c30e2a2730c206d65771cfa2302c90a2c90b0860207d82f041d24b7c16409e35465d2be987c4bf562734b9e62f248",
						origin:      "upload",
						revision:    "0",
					}),
				),
			},
			{
				// Remove the custom resource.
				Config: testAccResourceApplicationFromPrivateRegistry(modelName, appName, "", "", "74"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(appResourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApplicationResource(appResourceFullName, charmResourceChecks{
						fingerprint: "398048a2c483cd10a5e358f0d45ed8e21ed077079779fecce58772d443a3c9b53e871cf43dba94fcb3463adee154c440",
						origin:      "store",
						revision:    "74",
					}),
				),
			},
		},
	})
}

type charmResourceChecks struct {
	// fingerprint is a SHA356 fingerprint of the resource
	// composed from the image URL, username and password.
	// If we start uploading files, this would represent
	// the fingerprint of the file.
	fingerprint string
	// origin is either "store" or "upload".
	origin string
	// revision is "0" when origin is "store", otherwise it's the revision number.
	revision string
}

func testAccCheckApplicationResource(appResource string, checks charmResourceChecks) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// retrieve the resource by name from state
		rs, ok := s.RootModule().Resources[appResource]
		if !ok {
			return fmt.Errorf("Not found: %s", appResource)
		}

		model_uuid, ok := rs.Primary.Attributes["model_uuid"]
		if !ok {
			return fmt.Errorf("model_uuid is not set")
		}
		appName, ok := rs.Primary.Attributes["name"]
		if !ok {
			return fmt.Errorf("name is not set")
		}

		conn, err := TestClient.Models.GetConnection(&model_uuid)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		jc, err := resources.NewClient(conn)
		if err != nil {
			return err
		}

		resources, err := jc.ListResources([]string{appName})
		if err != nil {
			return err
		}
		if len(resources) != 1 || len(resources[0].Resources) != 1 {
			return fmt.Errorf("expected one resource for application %q, got %d", appName, len(resources))
		}
		resource := resources[0].Resources[0]
		if resource.Fingerprint.String() != checks.fingerprint {
			return fmt.Errorf("expected fingerprint %q, got %q", checks.fingerprint, resource.Fingerprint)
		}
		if resource.Origin.String() != checks.origin {
			return fmt.Errorf("expected origin %q, got %q", checks.origin, resource.Origin)
		}
		if strconv.Itoa(resource.Revision) != checks.revision {
			return fmt.Errorf("expected revision %q, got %q", checks.revision, resource.Revision)
		}
		return nil
	}
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
		resource.TestCheckResourceAttr(resourceName, "name", charmName),
		resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
		resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
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
		resource.TestCheckResourceAttrPair("juju_model.model", "uuid", ntpName, "model_uuid"),
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
			model_uuid = juju_model.model.uuid
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
	numberOfMachines := 10

	checkResourceAttrMachines := func(numberOfMachines int) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttrPair("juju_model.model", "uuid", resourceName, "model_uuid"),
			resource.TestCheckResourceAttr(resourceName, "name", charmName),
			resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
			resource.TestCheckResourceAttr(resourceName, "units", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(resourceName, "machines.#", fmt.Sprintf("%d", numberOfMachines)),
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
	numberOfMachines := 3

	checkResourceAttrMachines := func(numberOfMachines int) []resource.TestCheckFunc {
		return []resource.TestCheckFunc{
			resource.TestCheckResourceAttr(resourceName, "name", charmName),
			resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
			resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
			resource.TestCheckResourceAttr(resourceName, "units", fmt.Sprintf("%d", numberOfMachines)),
			resource.TestCheckResourceAttr(resourceName, "machines.#", fmt.Sprintf("%d", numberOfMachines)),
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
  			model_uuid = juju_model.model.uuid
			base = "ubuntu@22.04"
			name = "machine_${count.index}"

			# The following lifecycle directive instructs Terraform to create 
  			# new machines before destroying existing ones.
			lifecycle {
				create_before_destroy = true
			}
		}

		resource "juju_application" "testapp" {
		  name = "juju-qa-test"
		  model_uuid = juju_model.model.uuid


		  machines = toset( juju_machine.all_machines[*].machine_id )

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}

		resource "juju_application" "ntp" {
			model_uuid = juju_model.model.uuid
			name = "ntp"

			charm {
				name = "ntp"
				base = "ubuntu@22.04"
			}
		}

		resource "juju_integration" "testapp_ntp" {
			model_uuid = juju_model.model.uuid

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
  			model_uuid = juju_model.model.uuid
			base = "ubuntu@22.04"
			name = "machine_${count.index}"
		}

		resource "juju_application" "testapp" {
		  name = "juju-qa-test"
		  model_uuid = juju_model.model.uuid


		  units = %q

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}

		resource "juju_application" "ntp" {
			model_uuid = juju_model.model.uuid
			name = "ntp"

			charm {
				name = "ntp"
				base = "ubuntu@22.04"
			}
		}

		resource "juju_integration" "testapp_ntp" {
			model_uuid = juju_model.model.uuid

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
	checkResourceAttrMachines := []resource.TestCheckFunc{
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

func testAccResourceApplicationBasic_Machines(modelName, charmName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "machine" {
		  name = "test machine"
		  model_uuid = juju_model.model.uuid
		  base = "ubuntu@22.04"
		}

		resource "juju_application" "testapp" {
		  model_uuid = juju_model.model.uuid

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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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

func TestAcc_ResourceApplication_UpgradeV0ToV1(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")
	appName := "test-app"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderPreV1Version,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceApplicationVersioned(modelName, appName, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", "ubuntu-lite"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceApplicationVersioned(modelName, appName, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
					resource.TestCheckNoResourceAttr("juju_application.this", "model"),
				),
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

	modelUUID, managementSpace, publicSpace, cleanUp := setupModelAndSpaces(t, modelName)
	defer cleanUp()

	constraints := "arch=amd64 spaces=" + managementSpace + "," + publicSpace
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// test creating a single application with default endpoint bound to management space, and ubuntu endpoint bound to public space
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "ubuntu", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
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

	modelUUID, managementSpace, publicSpace, cleanUp := setupModelAndSpaces(t, modelName)
	defer cleanUp()
	constraints := "arch=amd64 spaces=" + managementSpace + "," + publicSpace

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// test creating a single application with default endpoint bound to management space
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, map[string]string{"": managementSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, managementSpace, map[string]string{"": managementSpace}),
				),
			},
			{
				// updating the existing application's default endpoint to be bound to public space
				// this means all endpoints should be bound to public space (since no endpoint was on a different space)
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, map[string]string{"": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "1"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, publicSpace, map[string]string{"": publicSpace, "ubuntu": publicSpace, "another": publicSpace}),
				),
			},
			{
				// updating the existing application's default endpoint to be bound to management space, and specifying ubuntu endpoint to be bound to public space
				// this means all endpoints should be bound to public space, except for ubuntu which should be bound to public space
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "", "space": managementSpace}),
					resource.TestCheckTypeSetElemNestedAttrs("juju_application."+appName, "endpoint_bindings.*", map[string]string{"endpoint": "ubuntu", "space": publicSpace}),
					testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace, "another": managementSpace}),
				),
			},
			{
				// removing the endpoint bindings reverts to model's default space
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, nil),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "0"),
					testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, "alpha", map[string]string{"": "alpha", "ubuntu": "alpha", "another": "alpha"}),
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
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
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
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
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

func TestAcc_ResourceApplicationChangingChannel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithChannelAndRevision(modelName, "latest/candidate", 20),
			},
			{
				Config: testAccResourceApplicationWithChannelAndRevision(modelName, "latest/stable", 20),
			},
		}})
}

func testAccApplicationConfigNull(modelName, appName, configValue string, includeConfig bool) string {
	return internaltesting.GetStringFromTemplateWithData("testAccApplicationConfigNull", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name       = "{{.AppName}}"
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
		  model_uuid = juju_model.testmodel.uuid
		  charm {
			name = %q
		  }
		}
		`, modelName, charmName)
}

func testAccResourceApplicationVersioned(modelName, appName string, version int) string {
	switch version {
	case 0:
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
			}
			`, modelName, appName)
	case 1:
		return fmt.Sprintf(`
			resource "juju_model" "this" {
			  name = %q
			}
			
			resource "juju_application" "this" {
			  model_uuid = juju_model.this.uuid
			  name = %q
			  charm {
				name = "ubuntu-lite"
			  }
			}
			`, modelName, appName)
	default:
		panic(fmt.Sprintf("Unsupported version %d", version))
	}
}

func testAccResourceApplicationBasic(modelName, appName string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.{{.ModelName}}.uuid

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

func testAccResourceApplicationFromPrivateRegistry(modelName, appName, username, password string, resourceRevision string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationFromPrivateRegistry",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  name  = "{{.AppName}}"
  model_uuid = juju_model.{{.ModelName}}.uuid

  charm {
    name     = "coredns"
    revision = 191
    channel  = "latest/stable"
  }

  {{ if .UsePrivateRegistry }}
  registry_credentials = {
    "ghcr.io/canonical" = {
      username = "{{.Username}}"
      password = "{{.Password}}"
    }
  }
  {{ end }}

  {{ if ne .ResourceParamRevision "" }}
  resources = {
    "coredns-image" = "{{.ResourceParamRevision}}"
  }
  {{ end }}

  units = 1
}
`, internaltesting.TemplateData{
			"ModelName":             modelName,
			"AppName":               appName,
			"UsePrivateRegistry":    (username != "" && password != ""),
			"Username":              username,
			"Password":              password,
			"ResourceParamRevision": resourceRevision,
		})
}

func testAccResourceApplicationWithCustomResources(modelName, channel string, resourceName string, customResource string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
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

func testAccResourceApplicationWithChannelAndRevision(modelName, channel string, revision int) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "test-app"
  charm {
    name     = "juju-qa-test"
	channel  = "%s"
	revision = %d
  }
}
`, modelName, channel, revision)
}

func testAccResourceApplicationWithoutCustomResources(modelName, channel string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
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
	exposeStr := `expose {}`
	if !expose {
		exposeStr = ""
	}

	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
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

func testAccResourceApplicationRefreshCharmUpdatesResources(modelName string, revision int) string {
	return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
		  name = "test-app"
		  charm {
			name     = "coredns"
			revision = %d
		  }
		  config = {
			juju-external-hostname="myhostname"
		  }
		}
		`, modelName, revision)
}

func testAccResourceApplicationUpdatesCharm(modelName string, channel string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
		  name = "test-app"
		  charm {
			name     = "coredns"
			channel = %q
		  }
		}
		`, modelName, channel)
	}
}

func testAccResourceApplicationUpdatesCharmWithRevision(modelName string, channel string, revision string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationUpdatesCharm", `
		resource "juju_model" "this" {
		  name = "{{.ModelName}}"
		}

		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
		  name       = "test-app"
		  charm {
			name    = "juju-qa-test"
			channel = "{{.Channel}}"
			{{- if .Revision }}
			revision = "{{.Revision}}"
			{{- end }}
		  }
		}
		`, internaltesting.TemplateData{
		"ModelName": modelName,
		"Channel":   channel,
		"Revision":  revision,
	})
}

func testAccApplicationUpdateBaseCharm(modelName string, base string) string {
	if testingCloud == LXDCloudTesting {
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
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
		  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.this.uuid
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
  model_uuid = %q
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
  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.this.uuid
  name = "test-subordinate"
  charm {
    name = "nrpe"
    revision = 96
    }
} 
`, modelName, constraints)
}

func setupModelAndSpaces(t *testing.T, modelName string) (string, string, string, func()) {
	// All the space setup is needed until https://github.com/juju/terraform-provider-juju/issues/336 is implemented
	// called to have TestClient populated
	testAccPreCheck(t)
	model, err := TestClient.Models.CreateModel(internaljuju.CreateModelInput{
		Name: modelName,
	})
	if err != nil {
		t.Fatal(err)
	}
	modelUUID := model.UUID

	conn, err := TestClient.Models.GetConnection(&model.UUID)
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

	return modelUUID, managementSpace, publicSpace, cleanUp
}

func testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints string, endpointBindings map[string]string) string {
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
  uuid = "{{.ModelUUID}}"
}

resource "juju_application" "{{.AppName}}" {
  model_uuid  = data.juju_model.{{.ModelName}}.uuid
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
		"ModelUUID":        modelUUID,
	})
}

func testAccResourceApplicationStorageLXD(modelName, appName string, storageConstraints map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "{{.AppName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
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
  model_uuid = juju_model.{{.ModelName}}.uuid
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

func testCheckEndpointsAreSetToCorrectSpace(modelUUID, appName, defaultSpace string, configuredEndpoints map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := TestClient.Models.GetConnection(&modelUUID)
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
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName1, "model_uuid"),
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName2, "model_uuid"),
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
  model_uuid = juju_model.{{.ModelName}}.uuid
  name = "{{.AppName1}}"
  charm {
    name = "{{.CharmName}}"
    channel = "{{.CharmChannel}}"
  }
  units = 1
}

resource "juju_application" "{{.AppName2}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
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
  model_uuid = juju_model.{{.ModelName}}.uuid
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
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
  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.this.uuid
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
  model_uuid = juju_model.this.uuid
  name = %q
  charm {
	name = "ubuntu-lite"
  }
  units = 1
  constraints = %q
}
		`, modelName, appName, constraints)
}

func TestAcc_ResourceApplicationInvalidModelUUID(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-application")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model_uuid = "invalid-uuid"
  name = "test-app"
  charm {
	name = "ubuntu-lite"
  }
}
`, modelName),
				ExpectError: regexp.MustCompile(`invalid-uuid`),
			},
		},
	})
}

func TestCreateCharmResources(t *testing.T) {
	tests := []struct {
		name          string
		planResources map[string]string
		registryCreds map[string]registryDetails
		expected      internaljuju.CharmResources
		expectError   bool
	}{
		{
			name: "Valid charm revision",
			planResources: map[string]string{
				"charm1": "123",
			},
			registryCreds: map[string]registryDetails{},
			expected: internaljuju.CharmResources{
				"charm1": {
					RevisionNumber: "123",
				},
			},
			expectError: false,
		},
		{
			name: "Valid OCI image URL with path",
			planResources: map[string]string{
				"charm2": "registry.example.com/path/image:tag",
			},
			registryCreds: map[string]registryDetails{
				"registry.example.com/path": {
					User:     types.StringValue("user"),
					Password: types.StringValue("pass"),
				},
			},
			expected: internaljuju.CharmResources{
				"charm2": {
					OCIImageURL:      "registry.example.com/path/image:tag",
					RegistryUser:     "user",
					RegistryPassword: "pass",
				},
			},
			expectError: false,
		},
		{
			name: "Valid OCI image URL with path that doesn't match registry",
			planResources: map[string]string{
				"charm2": "registry.example.com/path/image:tag",
			},
			registryCreds: map[string]registryDetails{
				"registry.example.com/anotherpath": {
					User:     types.StringValue("user"),
					Password: types.StringValue("pass"),
				},
			},
			expected: internaljuju.CharmResources{
				"charm2": {
					OCIImageURL: "registry.example.com/path/image:tag",
				},
			},
			expectError: false,
		},
		{
			name: "Multiple OCI images with different registries",
			planResources: map[string]string{
				"charm2": "registry.example.com/path/image:tag",
				"charm3": "another-registry.com/otherpath/image:tag",
			},
			registryCreds: map[string]registryDetails{
				"registry.example.com/path": {
					User:     types.StringValue("user"),
					Password: types.StringValue("pass"),
				},
				"another-registry.com/otherpath": {
					User:     types.StringValue("user2"),
					Password: types.StringValue("pass2"),
				},
			},
			expected: internaljuju.CharmResources{
				"charm2": {
					OCIImageURL:      "registry.example.com/path/image:tag",
					RegistryUser:     "user",
					RegistryPassword: "pass",
				},
				"charm3": {
					OCIImageURL:      "another-registry.com/otherpath/image:tag",
					RegistryUser:     "user2",
					RegistryPassword: "pass2",
				},
			},
			expectError: false,
		},
		{
			name: "Valid OCI image URL without path",
			planResources: map[string]string{
				"charm2": "registry.example.com/image:tag",
			},
			registryCreds: map[string]registryDetails{
				"registry.example.com": {
					User:     types.StringValue("user"),
					Password: types.StringValue("pass"),
				},
			},
			expected: internaljuju.CharmResources{
				"charm2": {
					OCIImageURL:      "registry.example.com/image:tag",
					RegistryUser:     "user",
					RegistryPassword: "pass",
				},
			},
			expectError: false,
		},
		{
			name: "Empty resource error",
			planResources: map[string]string{
				"charm3": "",
			},
			registryCreds: map[string]registryDetails{},
			expected:      nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createCharmResources(tt.planResources, tt.registryCreds)
			if (err != nil) != tt.expectError {
				t.Errorf("createCharmResources() error = %v, expectError %v", err, tt.expectError)
				return
			}

			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("createCharmResources() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestAcc_ResourceApplication_RemoveConfigNotExistingAnymore tests that removing a config entry from terraform that
// does not exist anymore in the juju application config.
func TestAcc_ResourceApplication_RemoveConfigNotExistingAnymore(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with MicroK8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-channel-revision")
	appName := "my-test-charm"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithChannelRevisionAndConfig(modelName, appName, "juju-jimm-k8s", "3/stable", 90, true),
			},
			{
				Config: testAccResourceApplicationWithChannelRevisionAndConfig(modelName, appName, "juju-jimm-k8s", "3/stable", 50, false),
			},
		},
	})
}

func testAccResourceApplicationWithChannelRevisionAndConfig(modelName, appName, charmName, channel string, revision int, configNew bool) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationWithChannelRevisionAndConfig",
		`
resource "juju_model" "development" {
  name = "{{.ModelName}}"
}

resource "juju_application" "test_app" {
  name       = "{{.AppName}}"
  model_uuid = juju_model.development.uuid

  charm {
    name     = "{{.CharmName}}"
    channel  = "{{.Channel}}"
    revision = {{.Revision}}
  }
  config = {
   {{- if .ConfigNew }}
    ssh-max-concurrent-connections = "1"
   {{- else }}
	dns-name = "test.localhost"
   {{- end }}
  }
 
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"AppName":   appName,
			"CharmName": charmName,
			"Channel":   channel,
			"Revision":  revision,
			"ConfigNew": configNew,
		})
}
