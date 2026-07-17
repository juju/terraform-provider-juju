// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/api/client/resources"
	apispaces "github.com/juju/juju/api/client/spaces"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

func TestAcc_ResourceApplication(t *testing.T) {
	SkipAgainstJuju4WithReason(t, "See  https://github.com/juju/juju/issues/21717")
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
    revision = 59
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

	charmName := "ubuntu-lite"
	if testingCloud != LXDCloudTesting {
		charmName = "coredns"
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccResourceApplicationScaleUp(modelName, appName, "1"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", charmName),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.0", "0"),
			),
		}, {
			Config: testAccResourceApplicationScaleUp(modelName, appName, "2"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", charmName),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.#", "2"),
			),
		}, {
			// Scale back down to 1. The remaining unit may not be /0
			// because Juju may have destroyed unit 0 and kept unit 1.
			Config: testAccResourceApplicationScaleUp(modelName, appName, "1"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_application.this", "model_uuid"),
				resource.TestCheckResourceAttr("juju_application.this", "name", appName),
				resource.TestCheckResourceAttr("juju_application.this", "charm.#", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", charmName),
				resource.TestCheckResourceAttr("juju_application.this", "trust", "true"),
				resource.TestCheckResourceAttr("juju_application.this", "units", "1"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.#", "1"),
			),
		}, {
			// Scale back up to 2. On IAAS, unit numbers are never reused, so
			// the new unit gets /2 (after /0 and /1 were taken). On CAAS,
			// unit numbers are not auto-incremented the same way, so the
			// new unit gets /1.
			Config: testAccResourceApplicationScaleUp(modelName, appName, "2"),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("juju_application.this", "units", "2"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.#", "2"),
				resource.TestCheckResourceAttr("juju_application.this", "unit_numbers.1", expectedSecondUnitNumber()),
			),
		}},
	})
}

// expectedSecondUnitNumber returns the unit number expected for the second
// unit after a scale-down-then-up cycle. On IAAS, Juju never reuses unit
// numbers, so after /0 and /1 were taken the new unit gets /2. On CAAS,
// unit numbers are not auto-incremented the same way, so the new unit gets /1.
func expectedSecondUnitNumber() string {
	if testingCloud != LXDCloudTesting {
		return "1"
	}
	return "2"
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
				Check:  resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "191"),
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
				Config: testAccResourceApplicationRefreshCharmUpdatesResources(modelName, 191),
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
						err := TestClient.Applications.UpdateApplication(t.Context(), &input)
						if err != nil {
							return err
						}
						// Read the application to verify the resource revision is set to 60
						// and the charm revision is 191.
						readInput := internaljuju.ReadApplicationInput{
							ModelUUID: modelUUID,
							AppName:   "test-app",
						}
						readRes, err := TestClient.Applications.ReadApplication(t.Context(), &readInput)
						if err != nil {
							return err
						}
						if readRes.Revision != 191 {
							return fmt.Errorf("expected charm revision to be 191, got %d", readRes.Revision)
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
				Config: testAccResourceApplicationRefreshCharmUpdatesResources(modelName, 199),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.revision", "199"),
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
						res, err := TestClient.Applications.ReadApplication(t.Context(), &input)
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

	resp, err := TestClient.Models.CreateModel(
		t.Context(),
		internaljuju.CreateModelInput{
			Name: modelName,
		},
	)
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
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, "latest/edge", 88, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.#", "1"),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.name", appName),
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.revision", "88"),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, "latest/edge", 96, configParamName, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application."+appName, "charm.0.revision", "96"),
					resource.TestCheckResourceAttr("juju_application."+appName, "config."+configParamName, configParamName+"-value"),
				),
			},
		},
	})
}

func TestAcc_CharmUpdatesNoRevision(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")
	initialVersion := 0
	channelOne := "latest/stable"
	channelTwo := "2.0/stable"
	if testingCloud == MicroK8sTesting {
		channelOne = "1.34/stable"
		channelTwo = "1.35/stable"
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationUpdatesCharm(modelName, channelOne),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", channelOne),
					func(s *terraform.State) error {
						// Use a check to grab the application revision and set it to initialVersion variable to be used in the next step.
						rs, ok := s.RootModule().Resources["juju_application.this"]
						if !ok {
							return fmt.Errorf("not found: juju_application.this")
						}
						initialVersionStr := rs.Primary.Attributes["charm.0.revision"]
						var err error
						initialVersion, err = strconv.Atoi(initialVersionStr)
						if err != nil {
							return fmt.Errorf("error converting revision to int: %v", err)
						}
						if initialVersion == 0 {
							return fmt.Errorf("expected initial charm revision to be non-zero, got %d", initialVersion)
						}
						return nil
					},
				),
			},
			{
				// move to a new channel
				PreConfig: func() {
					// This sleep is necessary because without it, Juju does not update the charm revision and the test fails.
					// It is unclear where exactly the problem is, but it is almost certainly in the computeCharmID logic,
					// specifically the resolveCharm function, which calls charmsAPIClient.ResolveCharms so possibly
					// (likely) a race condition/timing issue on the controller side.
					time.Sleep(15 * time.Second)
				},
				Config: testAccResourceApplicationUpdatesCharm(modelName, channelTwo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", channelTwo),
					func(s *terraform.State) error {
						// Check that the charm revision has been updated to a new revision different from the initialVersion.
						rs, ok := s.RootModule().Resources["juju_application.this"]
						if !ok {
							return fmt.Errorf("not found: juju_application.this")
						}
						newVersionStr := rs.Primary.Attributes["charm.0.revision"]
						var err error
						newVersion, err := strconv.Atoi(newVersionStr)
						if err != nil {
							return fmt.Errorf("error converting revision to int: %v", err)
						}
						if newVersion == initialVersion {
							return fmt.Errorf("expected charm revision to be updated from %d, but it is still %d", initialVersion, newVersion)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestAcc_ConfigChangeKeepsCharm tests that when a field on a juju_application resource
// is changed, that is not related to the charm, it does not trigger a charm refresh.
func TestAcc_ConfigChangeKeepsCharm(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-charmupdates")
	initialVersion := 0

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationCharmWithRevisionAndConfig(modelName, "latest/stable", "20", nil),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "latest/stable"),
					func(s *terraform.State) error {
						// Use a check to grab the application revision and set it to initialVersion variable to be used in the next step.
						rs, ok := s.RootModule().Resources["juju_application.this"]
						if !ok {
							return fmt.Errorf("not found: juju_application.this")
						}
						initialVersionStr := rs.Primary.Attributes["charm.0.revision"]
						var err error
						initialVersion, err = strconv.Atoi(initialVersionStr)
						if err != nil {
							return fmt.Errorf("error converting revision to int: %v", err)
						}
						if initialVersion == 0 {
							return fmt.Errorf("expected initial charm revision to be non-zero, got %d", initialVersion)
						}
						return nil
					},
				),
			},
			{
				// Remove the revision from the plan.
				Config: testAccResourceApplicationCharmWithRevisionAndConfig(modelName, "latest/stable", "", nil),
				PreConfig: func() {
					// This sleep is necessary because without it, Juju does not update the charm revision and the test fails.
					// It is unclear where exactly the problem is, but it is almost certainly in the computeCharmID logic,
					// specifically the resolveCharm function, which calls charmsAPIClient.ResolveCharms so possibly
					// (likely) a race condition/timing issue on the controller side.
					time.Sleep(15 * time.Second)
				},
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				// Add a config key the plan.
				Config: testAccResourceApplicationCharmWithRevisionAndConfig(modelName, "latest/stable", "", map[string]string{"thing": "foo"}),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.channel", "latest/stable"),
					// Check that the config change is in the state.
					resource.TestCheckResourceAttr("juju_application.this", "config.thing", "foo"),
					func(s *terraform.State) error {
						// Check that the charm revision remains the same.
						rs, ok := s.RootModule().Resources["juju_application.this"]
						if !ok {
							return fmt.Errorf("not found: juju_application.this")
						}
						currentVersionStr := rs.Primary.Attributes["charm.0.revision"]
						var err error
						currentVersion, err := strconv.Atoi(currentVersionStr)
						if err != nil {
							return fmt.Errorf("error converting revision to int: %v", err)
						}
						if currentVersion != initialVersion {
							return fmt.Errorf("expected charm revision to remain the same, but it changed from %d to %d", initialVersion, currentVersion)
						}
						return nil
					},
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
	t.Skip(t.Name() + " Waiting on issue 21717 for LXD, and PR 22237 for K8s to be resolved")

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
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 21, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
			{
				// change resource revision to 3
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 21, "", "foo-file", "3"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "3"),
				),
			},
			{
				// change back to 4
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 21, "", "foo-file", "4"),
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
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 20, "", "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.juju-qa-test", "resources"),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 21, "", "foo-file", "4"),
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
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 20, "", "foo-file", "4"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.juju-qa-test", "resources.foo-file", "4"),
				),
			},
			{
				// then remove the resource revision and update the charm revision
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, "juju-qa-test", "latest/edge", 21, "", "", ""),
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
	appName := "coredns"
	appResourceName := "juju_application." + appName

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, "latest/stable", 191, "", "coredns-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(appResourceName, "resources.coredns-image", "59"),
					testAccCheckApplicationIdle(t.Context(), appResourceName),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, "latest/stable", 191, "", "coredns-image", "60"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(appResourceName, "resources.coredns-image", "60"),
					testAccCheckApplicationIdle(t.Context(), appResourceName),
				),
			},
			{
				Config: testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, "latest/stable", 191, "", "coredns-image", "59"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(appResourceName, "resources.coredns-image", "59"),
					testAccCheckApplicationIdle(t.Context(), appResourceName),
				),
			},
		},
	})
}

func testAccCheckApplicationIdle(ctx context.Context, appResource string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[appResource]
		if !ok {
			return fmt.Errorf("not found: %s", appResource)
		}

		modelUUID, ok := rs.Primary.Attributes["model_uuid"]
		if !ok {
			return fmt.Errorf("model_uuid is not set")
		}
		appName, ok := rs.Primary.Attributes["name"]
		if !ok {
			return fmt.Errorf("name is not set")
		}

		return internaltesting.WaitForApplicationIdle(ctx, TestClient.Models, modelUUID, appName)
	}
}

// testAccCheckImagePullSecretCreated verifies that Juju created a k8s
// dockerconfigjson image pull secret in the model's namespace for the given
// application. This confirms that the private registry credentials were
// correctly marshaled and uploaded to the controller, and that the secret
// content contains the expected username and password.
//
// The k8s namespace is the model name and the secret name is
// "<app-name>-<container-name>-secret".
func testAccCheckImagePullSecretCreated(ctx context.Context, appResource, modelName, containerName, expectedUsername, expectedPassword string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[appResource]
		if !ok {
			return fmt.Errorf("not found: %s", appResource)
		}
		appName, ok := rs.Primary.Attributes["name"]
		if !ok {
			return fmt.Errorf("name is not set")
		}
		namespace := modelName
		secretName := fmt.Sprintf("%s-%s-secret", appName, containerName)
		// Poll for the secret, since Juju creates it asynchronously after the
		// resource upload is processed by the controller.
		_, err := wait.WaitFor(wait.WaitForCfg[struct{}, struct{}]{
			Context: ctx,
			GetData: func(ctx context.Context, _ struct{}) (struct{}, error) {
				secret, err := TestK8sClientset.CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
				if err != nil {
					return struct{}{}, internaljuju.NewRetryReadErrorf(
						"image pull secret %q not found in namespace %q yet: %v",
						secretName, namespace, err,
					)
				}
				if secret.Type != "kubernetes.io/dockerconfigjson" {
					return struct{}{}, fmt.Errorf("expected secret %q to be of type kubernetes.io/dockerconfigjson, got %s",
						secretName, secret.Type)
				}
				dockerConfigJSON, ok := secret.Data[".dockerconfigjson"]
				if !ok {
					return struct{}{}, fmt.Errorf("secret %q does not contain .dockerconfigjson data", secretName)
				}
				// Verify the credentials are present in the secret content.
				var dockerConfig struct {
					Auths map[string]struct {
						Username string `json:"Username"`
						Password string `json:"Password"`
					} `json:"auths"`
				}
				if err := json.Unmarshal(dockerConfigJSON, &dockerConfig); err != nil {
					return struct{}{}, fmt.Errorf("failed to unmarshal .dockerconfigjson for secret %q: %w", secretName, err)
				}
				found := false
				for _, auth := range dockerConfig.Auths {
					if auth.Username == expectedUsername && auth.Password == expectedPassword {
						found = true
						break
					}
				}
				if !found {
					return struct{}{}, internaljuju.NewRetryReadErrorf(
						"expected username %q and password %q not found in secret %q dockerconfigjson",
						expectedUsername, expectedPassword, secretName,
					)
				}
				return struct{}{}, nil
			},
			Input:          struct{}{},
			NonFatalErrors: []error{internaljuju.RetryReadError},
			RetryConf: &wait.RetryConf{
				Delay:       1 * time.Second,
				MaxDelay:    5 * time.Second,
				MaxDuration: 1 * time.Minute,
			},
		})
		return err
	}
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
			// In the next step we verify the plan has no changes. Wait for idle
			// to avoid a race condition in Juju where updating the resource revision too
			// quickly means that the change doesn't take immediate effect.
			{
				Config: testAccResourceApplicationWithoutCustomResources(modelName, "latest/stable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_application.this", "resources"),
				),
				PreConfig: func() {
					if err := testAccWaitForApplicationIdle(t.Context(), modelName, "test-app"); err != nil {
						t.Fatal(err)
					}
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
				// We wait for idleness. Otherwise, after we try to destroy the application the
				// agent can go into `lost` state, making the test waits on application destroy
				// until the timeout is reached.
				// This is not an issue because if we reach the timeout we don't error out,
				// but it slows down the test suite.
				PreConfig: func() {
					if err := testAccWaitForApplicationIdle(t.Context(), modelName, "test-app"); err != nil {
						t.Fatal(err)
					}
				},
			},
		},
	})
}

func testAccWaitForApplicationIdle(ctx context.Context, modelName, appName string) error {
	modelUUIDs, err := TestClient.Models.ListModels(ctx)
	if err != nil {
		return err
	}

	var modelUUID string
	for _, candidateUUID := range modelUUIDs {
		model, err := TestClient.Models.ReadModel(ctx, candidateUUID)
		if err != nil {
			return err
		}
		if model.ModelInfo.Name == modelName {
			modelUUID = candidateUUID
			break
		}
	}
	if modelUUID == "" {
		return fmt.Errorf("model %q not found", modelName)
	}

	return internaltesting.WaitForApplicationIdle(ctx, TestClient.Models, modelUUID, appName)
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
	ctx := t.Context()

	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName := acctest.RandomWithPrefix("tf-test-custom-resource-file")
	appName := "test-app"
	appResourceFullName := "juju_application." + appName
	// - Remove the custom resource.

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckWithK8s(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// A custom resource from a private registry.
				Config: testAccResourceApplicationFromPrivateRegistry(modelName, appName, "user", "pass", "ghcr.io/canonical/test:dfb5e3fa84d9476c492c8693d7b2417c0de8742f"),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckApplicationResource(ctx, appResourceFullName, charmResourceChecks{
						fingerprintJuju3: "1b94afe549b44328f2350ae24633b31265a01e466cf0469faa798acb9c637bea30c3c711f25937795eff34d2f920e074",
						fingerprintJuju4: "2f4df0e226b8e3599dc0e7cae663046fb113e155e72f25325d02c9671f9eb9fd61ddb75c2958cef850745b94431d44b8",
						origin:           "upload",
						revision:         "0",
					}),
					// Verify that Juju created an image pull secret in the
					// model's k8s namespace with the correct credentials.
					testAccCheckImagePullSecretCreated(ctx, appResourceFullName, modelName, "coredns", "user", "pass"),
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
					testAccCheckApplicationResource(ctx, appResourceFullName, charmResourceChecks{
						fingerprintJuju3: "953991156cf1e0a601f52b2b2b16c7042ad13bf765655c024f384385306404b7eb30bf72bdfcfda3c570b076b3aa96dc",
						fingerprintJuju4: "33d64f169e84ad1ba0f8ebcb6f5e8c1a135b85a9115b3f5c9f664b013b8facb46dfe0493402d4865e4eebc2843fbca15",
						origin:           "upload",
						revision:         "0",
					}),
					// Verify the secret was updated with the new credentials.
					testAccCheckImagePullSecretCreated(ctx, appResourceFullName, modelName, "coredns", "user2", "pass2"),
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
					testAccCheckApplicationResource(ctx, appResourceFullName, charmResourceChecks{
						fingerprintJuju3: "591c30e2a2730c206d65771cfa2302c90a2c90b0860207d82f041d24b7c16409e35465d2be987c4bf562734b9e62f248",
						fingerprintJuju4: "fc15a3374f0051849b218544ad39e3c6b6446d7ac411a9843c0f0f7102587219c1716dd4a217fbba1519b182e89cfda9",
						origin:           "upload",
						revision:         "0",
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
					testAccCheckApplicationResource(ctx, appResourceFullName, charmResourceChecks{
						fingerprintJuju3: "398048a2c483cd10a5e358f0d45ed8e21ed077079779fecce58772d443a3c9b53e871cf43dba94fcb3463adee154c440",
						fingerprintJuju4: "",
						origin:           "store",
						revision:         "74",
					}),
				),
			},
		},
	})
}

type charmResourceChecks struct {
	// fingerprintJuju3 is a SHA384 fingerprint of the resource as computed by Juju 3.
	// Juju 3 and Juju 4 compute fingerprints differently for container image resources
	// because Juju 4 re-serializes the image metadata as JSON while Juju 3 stores the
	// raw uploaded blob.
	fingerprintJuju3 string
	// fingerprintJuju4 is a SHA384 fingerprint of the resource as computed by Juju 4.
	fingerprintJuju4 string
	// origin is either "store" or "upload".
	origin string
	// revision is "0" when origin is "store", otherwise it's the revision number.
	revision string
}

func testAccCheckApplicationResource(ctx context.Context, appResource string, checks charmResourceChecks) resource.TestCheckFunc {
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

		conn, err := TestClient.Models.GetConnection(ctx, &model_uuid)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		jc, err := resources.NewClient(conn)
		if err != nil {
			return err
		}

		resources, err := jc.ListResources(context.TODO(), []string{appName})
		if err != nil {
			return err
		}
		if len(resources) != 1 || len(resources[0].Resources) != 1 {
			return fmt.Errorf("expected one resource for application %q, got %d", appName, len(resources))
		}
		resource := resources[0].Resources[0]
		expectedFingerprint := checks.fingerprintJuju3
		if internaltesting.CompareVersions(os.Getenv(TestJujuAgentVersion), "4.0.0") >= 0 {
			expectedFingerprint = checks.fingerprintJuju4
		}
		if resource.Fingerprint.String() != expectedFingerprint {
			return fmt.Errorf("expected fingerprint %q, got %q", expectedFingerprint, resource.Fingerprint)
		}
		if resource.Origin.String() != checks.origin {
			return fmt.Errorf("expected origin %q, got %q", checks.origin, resource.Origin)
		}
		if strconv.Itoa(resource.Revision) != checks.revision {
			return fmt.Errorf("expected revision %q, got %d", checks.revision, resource.Revision)
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
	numberOfMachines := 3

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
				"machines": config.IntegerVariable(1),
			},
			Config: testAccResourceApplicationBasic_MachinesWithSubordinates(modelName, charmName),
			Check: resource.ComposeTestCheckFunc(
				checkResourceAttrMachines(1)...),
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
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Start with 2 machines
			{
				Config: testAccResourceApplicationBasic_Machines(modelName, charmName, 2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", charmName),
					resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
					resource.TestCheckResourceAttr(resourceName, "units", "2"),
					resource.TestCheckResourceAttr(resourceName, "machines.0", "0"),
					resource.TestCheckResourceAttr(resourceName, "machines.1", "1"),
				),
			},
			// Scale down to 1 machine — exercises RemoveUnitsFromMachine before machine destroy
			{
				Config: testAccResourceApplicationBasic_Machines(modelName, charmName, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", charmName),
					resource.TestCheckResourceAttr(resourceName, "charm.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "charm.0.name", charmName),
					resource.TestCheckResourceAttr(resourceName, "units", "1"),
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

func testAccResourceApplicationBasic_Machines(modelName, charmName string, machineCount int) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}

		resource "juju_machine" "machine" {
		  count      = %d
		  model_uuid = juju_model.model.uuid
		  base       = "ubuntu@22.04"
		}

		resource "juju_application" "testapp" {
		  model_uuid = juju_model.model.uuid

		  machines = [for m in juju_machine.machine : m.machine_id]

		  charm {
			name = %q
			base = "ubuntu@22.04"
		  }
		}
		`, modelName, machineCount, charmName)
}

func TestAcc_ResourceApplication_UpgradeProvider(t *testing.T) {
	// This skip is temporary until we have a stable version of the provider that supports
	// Juju 4.0.0 and above, at which point we can re-enable it.
	SkipAgainstJuju4(t)
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

func TestAcc_ResourceApplication_EndpointBindings(t *testing.T) {
	ctx := t.Context()

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
					testCheckEndpointsAreSetToCorrectSpace(ctx, modelUUID, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace}),
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
	SkipAgainstJuju4WithReason(t, "See https://github.com/juju/juju/issues/22233.")
	ctx := t.Context()

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
					testCheckEndpointsAreSetToCorrectSpace(ctx, modelUUID, appName, managementSpace, map[string]string{"": managementSpace}),
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
					testCheckEndpointsAreSetToCorrectSpace(ctx, modelUUID, appName, publicSpace, map[string]string{"": publicSpace, "ubuntu": publicSpace, "another": publicSpace}),
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
					testCheckEndpointsAreSetToCorrectSpace(ctx, modelUUID, appName, managementSpace, map[string]string{"": managementSpace, "ubuntu": publicSpace, "another": managementSpace}),
				),
			},
			{
				// removing the endpoint bindings reverts to model's default space
				Config: testAccResourceApplicationEndpointBindings(modelName, modelUUID, appName, constraints, nil),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_model."+modelName, "uuid", "juju_application."+appName, "model_uuid"),
					resource.TestCheckResourceAttr("juju_application."+appName, "endpoint_bindings.#", "0"),
					testCheckEndpointsAreSetToCorrectSpace(ctx, modelUUID, appName, "alpha", map[string]string{"": "alpha", "ubuntu": "alpha", "another": "alpha"}),
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
	// Storage is not supported in Juju 4.
	SkipAgainstJuju4(t)

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
	// Storage is not supported in Juju 4.
	SkipAgainstJuju4(t)
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
			%s
		  }
		}
		`, modelName, appName, jujuExternalHostname())
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
		// For K8s (CAAS) we need a charm that supports scaling on Kubernetes.
		return fmt.Sprintf(`
		resource "juju_model" "this" {
		  name = %q
		}
		
		resource "juju_application" "this" {
		  model_uuid = juju_model.this.uuid
		  name = %q
		  charm {
			name = "coredns"
		  }
		  trust = true
		  units = %q
		}
		`, modelName, appName, numberOfUnits)
	}
}

func testAccResourceApplicationWithRevisionChannelAndConfig(modelName, appName, channel string, revision int, configParamName string, resourceName string, resourceRevision string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceApplicationWithRevisionChannelAndConfig",
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
		channel  = "{{.Channel}}"
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
			"Channel":               channel,
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
    %s
  }
}
`, modelName, channel, resourceName, customResource, jujuExternalHostname())
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
    %s
  }
}
`, modelName, channel, jujuExternalHostname())
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
			revision = 191
		  }
		  trust = true
		  %s
		  config = {
		  	# hostname = "%s"
			%s
		  }
		}
		`, modelName, units, exposeStr, hostname, jujuExternalHostname())
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
			name     = "juju-qa-test"
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

func testAccResourceApplicationCharmWithRevisionAndConfig(modelName, channel, revision string, config map[string]string) string {
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
		  {{- if .Config }}
		  config = {
			{{- range $key, $value := .Config }}
			{{$key}} = "{{$value}}"
			{{- end }}
		  }
		  {{- end }}
		}
		`, internaltesting.TemplateData{
		"ModelName": modelName,
		"Channel":   channel,
		"Revision":  revision,
		"Config":    config,
	})
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
    %s
  }
}
`, modelName, constraints, jujuExternalHostname())
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
	model, err := TestClient.Models.CreateModel(t.Context(), internaljuju.CreateModelInput{
		Name: modelName,
	})
	if err != nil {
		t.Fatal(err)
	}
	modelUUID := model.UUID

	conn, err := TestClient.Models.GetConnection(t.Context(), &model.UUID)
	if err != nil {
		t.Fatal(err)
	}
	cleanUp := func() {
		_ = TestClient.Models.DestroyModel(t.Context(), internaljuju.DestroyModelInput{UUID: model.UUID})
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
	err = spaceAPIClient.CreateSpace(t.Context(), managementSpace, []string{managementBridgeCidr}, true)
	if err != nil {
		t.Fatal(err)
	}
	err = spaceAPIClient.CreateSpace(t.Context(), publicSpace, []string{publicBridgeCidr}, true)
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

func testCheckEndpointsAreSetToCorrectSpace(ctx context.Context, modelUUID, appName, defaultSpace string, configuredEndpoints map[string]string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		conn, err := TestClient.Models.GetConnection(ctx, &modelUUID)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		applicationAPIClient := apiapplication.NewClient(conn)
		clientAPIClient := apiclient.NewClient(conn, TestClient.Applications.JujuLogger())

		apps, err := applicationAPIClient.ApplicationsInfo(
			context.Background(),
			[]names.ApplicationTag{names.NewApplicationTag(appName)},
		)
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
			status, err := clientAPIClient.Status(
				context.Background(),
				&apiclient.StatusArgs{})
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
  # No units needed: test only checks model_uuid attribute pair.
  units = 0
}

resource "juju_application" "{{.AppName2}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name = "{{.AppName2}}"
  charm {
    name = "{{.CharmName}}"
    channel = "{{.CharmChannel}}"
  }
  # No units needed: test only checks model_uuid attribute pair.
  units = 0
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
  # No units needed: test only checks config and trust attributes.
  units = 0
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
  # No units needed: test only checks config and trust attributes.
  units = 0
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
  # No units needed: test only checks that the apply succeeds.
  units = 0
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

// TestAcc_ResourceApplication_UnknownMachinesUnitsDeferred verifies that when
// the machines set is unknown at plan time (simulated via terraform_data whose
// output is not yet known), the units attribute is also marked unknown rather
// than incorrectly defaulting to 1.
func TestAcc_ResourceApplication_UnknownMachinesUnitsDeferred(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-application-unknown-machines")
	resourceName := "juju_application.testapp"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// On the first plan the terraform_data output is unknown, so
				// toset(...) produces an unknown set — exactly what happens when
				// machine IDs come from a not-yet-applied juju_machine inside a
				// module. The modifier must defer units (mark it unknown) rather
				// than falling through to the default of 1.
				Config: testAccResourceApplicationUnknownMachines(modelName),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectUnknownValue(resourceName, tfjsonpath.New("units")),
					},
				},
			},
			{
				// After apply the machines are known; units must equal the
				// number of machines (2) and the plan must be stable — i.e.
				// the modifier must NOT drift units back to 1 from the state
				// value of 2.
				Config: testAccResourceApplicationUnknownMachines(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "units", "2"),
				),
			},
		},
	})
}

func testAccResourceApplicationUnknownMachines(modelName string) string {
	return fmt.Sprintf(`
		resource "juju_model" "model" {
		  name = %q
		}
		resource "juju_machine" "machine1" {
		  model_uuid = juju_model.model.uuid
		  base       = "ubuntu@22.04"
		}
		resource "juju_machine" "machine2" {
		  model_uuid = juju_model.model.uuid
		  base       = "ubuntu@22.04"
		}
		# terraform_data.output is unknown on the first plan because it mirrors
		# its input, which depends on juju_machine.machine.machine_id that has
		# not been applied yet. This produces an unknown set for machines.
		resource "terraform_data" "machine_ids" {
		  input = [juju_machine.machine1.machine_id, juju_machine.machine2.machine_id]
		}
		resource "juju_application" "testapp" {
		  model_uuid = juju_model.model.uuid
		  machines   = toset(terraform_data.machine_ids.output)
		  charm {
			name = "juju-qa-test"
			base = "ubuntu@22.04"
		  }
		}
		`, modelName)
}

// TestAcc_ResourceApplication_LocalCharm_Deploy covers the full lifecycle of a
// locally-deployed charm:
//  1. Initial deploy: local_path_hash (full SHA-256) is populated in state.
//  2. Idempotency: a second apply with the same file produces no plan diff.
//  3. In-place refresh: rebuilding the archive with different content and path
//     (same charm name) triggers an update, not a replace.
//  4. Import round-trip: local_path and local_path_hash are excluded (the
//     controller does not record them).
func TestAcc_ResourceApplication_LocalCharm_Deploy(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-local-charm")
	appName := "local-test"
	charmName := "local-test-charm"

	dir := t.TempDir()
	archiveV1 := buildLocalCharm(t, filepath.Join(dir, "v1"), charmName, "version-1-content")
	archiveV2 := buildLocalCharm(t, filepath.Join(dir, "v2"), charmName, "version-2-content")

	var hashAfterV1 string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Step 1: deploy from local charm archive.
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", charmName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", archiveV1),
					// local_path_hash is the full SHA-256 (64 hex chars).
					resource.TestCheckResourceAttrWith(
						"juju_application.this", "charm.0.local_path_hash",
						func(value string) error {
							if len(value) != 64 {
								return fmt.Errorf("expected 64-char SHA-256, got %d chars: %q", len(value), value)
							}
							hashAfterV1 = value
							return nil
						},
					),
				),
			},
			{
				// Step 2: re-apply with the same config — no changes expected.
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
			{
				// Step 3: point local_path at the v2 archive.  Same charm name,
				// different content → in-place refresh (Update, not Replace).
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV2),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"juju_application.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", archiveV2),
					// Hash must have changed from step 1.
					resource.TestCheckResourceAttrWith(
						"juju_application.this", "charm.0.local_path_hash",
						func(value string) error {
							if len(value) != 64 {
								return fmt.Errorf("expected 64-char SHA-256, got %d chars", len(value))
							}
							if value == hashAfterV1 {
								return fmt.Errorf("local_path_hash unchanged after rebuilding charm")
							}
							return nil
						},
					),
				),
			},
			{
				// Step 4: import round-trip. local_path and local_path_hash are
				// not recoverable from the controller so they are excluded from
				// import verification.
				ImportState:             true,
				ImportStateVerify:       true,
				ResourceName:            "juju_application.this",
				ImportStateVerifyIgnore: []string{"charm.0.local_path", "charm.0.local_path_hash"},
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_RelativePath verifies that a
// local_path expressed relative to the Terraform working directory (rather
// than an absolute path) is resolved and deployed correctly, matching how the
// juju CLI resolves a local charm path against its shell working directory.
func TestAcc_ResourceApplication_LocalCharm_RelativePath(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-local-charm-relpath")
	appName := "local-relpath"
	charmName := "local-relpath-charm"

	dir := t.TempDir()
	archive := buildLocalCharm(t, dir, charmName, "relative-path-content")

	// Express the archive path relative to the process working directory (the
	// directory the provider resolves relative local_path values against).
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	relArchive, err := filepath.Rel(wd, archive)
	if err != nil {
		t.Fatalf("computing relative path: %v", err)
	}
	// Guard the premise of the test: the path must actually be relative.
	if filepath.IsAbs(relArchive) {
		t.Fatalf("expected a relative path, got %q", relArchive)
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, relArchive),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "name", appName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.name", charmName),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", relArchive),
					// local_path_hash is the full SHA-256 (64 hex chars),
					// proving the relative path was resolved and read.
					resource.TestCheckResourceAttrWith(
						"juju_application.this", "charm.0.local_path_hash",
						func(value string) error {
							if len(value) != 64 {
								return fmt.Errorf("expected 64-char SHA-256, got %d chars: %q", len(value), value)
							}
							return nil
						},
					),
				),
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_Drift verifies out-of-band charm
// drift detection for local charms: deploy v1, refresh to v2 directly via the
// Juju client, and confirm the next apply re-uploads v1
func TestAcc_ResourceApplication_LocalCharm_Drift(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	// Origin hash is not populated on older versions
	skipTestIfJujuAgentVersionBelow(t, internaljuju.LocalCharmOriginHashFirstAgentVersion)

	modelName := acctest.RandomWithPrefix("tf-test-local-charm-drift")
	appName := "local-drift"
	charmName := "local-drift-charm"

	dir := t.TempDir()
	archiveV1 := buildLocalCharm(t, filepath.Join(dir, "v1"), charmName, "drift-version-1")
	archiveV2 := buildLocalCharm(t, filepath.Join(dir, "v2"), charmName, "drift-version-2")

	var driftedOriginHash string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Deploy v1 with Terraform, then deploy v2 out of band.
				Config:             testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV1),
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", archiveV1),
					resource.TestCheckResourceAttrSet("juju_application.this", "charm.0.origin_hash"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["juju_model.this"]
						if !ok {
							return fmt.Errorf("not found: juju_model.this")
						}
						modelUUID := rs.Primary.Attributes["uuid"]
						// Add model to TestClient's model cache since it wasn't made with it.
						TestClient.Applications.AddModel(
							rs.Primary.Attributes["name"],
							"",
							modelUUID,
							model.ModelType(rs.Primary.Attributes["type"]),
						)
						input := internaljuju.UpdateApplicationInput{
							ModelUUID:      modelUUID,
							AppName:        appName,
							CharmLocalPath: archiveV2,
							Base:           "ubuntu@22.04",
						}
						if err := TestClient.Applications.UpdateApplication(t.Context(), &input); err != nil {
							return fmt.Errorf("out-of-band charm refresh failed: %w", err)
						}

						// Get the drifter origin hash for later check.
						readResp, err := TestClient.Applications.ReadApplication(t.Context(), &internaljuju.ReadApplicationInput{
							ModelUUID: modelUUID,
							AppName:   appName,
						})
						if err != nil {
							return fmt.Errorf("reading drifted application: %w", err)
						}
						driftedOriginHash = readResp.OriginHash
						if driftedOriginHash == "" {
							return fmt.Errorf("expected drifted origin hash to be set after out-of-band refresh")
						}
						return nil
					},
				),
			},
			{
				// Re-apply unchanged config. Drift is detected and v1 re-uploaded.
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectNonEmptyPlan(),
						plancheck.ExpectResourceAction(
							"juju_application.this",
							plancheck.ResourceActionUpdate,
						),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", archiveV1),
					resource.TestCheckResourceAttrSet("juju_application.this", "charm.0.origin_hash"),
					func(s *terraform.State) error {
						rs, ok := s.RootModule().Resources["juju_application.this"]
						if !ok {
							return fmt.Errorf("not found: juju_application.this")
						}
						got := rs.Primary.Attributes["charm.0.origin_hash"]
						if got == driftedOriginHash {
							return fmt.Errorf("expected reconciled origin_hash %q to differ from drifted v2 origin_hash", got)
						}
						return nil
					},
				),
			},
			{
				// A further apply is a no-op, confirming the drift was fixed.
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archiveV1),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_DriftUnsupported verifies that
// deploying a local charm against a controller that does not report a charm
// origin hash still succeeds, but leaves origin_hash empty (drift detection
// disabled). A warning is logged in that case; the deploy is not failed.
func TestAcc_ResourceApplication_LocalCharm_DriftUnsupported(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	// Only controllers this old don't populate the hash.
	skipTestIfJujuAgentVersionAtLeast(t, internaljuju.LocalCharmOriginHashFirstAgentVersion)

	modelName := acctest.RandomWithPrefix("tf-test-local-charm-drift-unsupported")
	appName := "local-drift-unsupported"
	charmName := "local-drift-unsupported-charm"

	dir := t.TempDir()
	archive := buildLocalCharm(t, dir, charmName, "v1")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// The deploy succeeds despite the missing hash; origin_hash is
				// empty, so out-of-band drift detection is disabled.
				Config: testAccResourceApplicationLocalCharm(modelName, appName, charmName, archive),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.local_path", archive),
					resource.TestCheckResourceAttr("juju_application.this", "charm.0.origin_hash", ""),
				),
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_NameMismatch verifies that
// ValidateConfig rejects a charm block where the declared name does not match
// the charm name in the archive's metadata.yaml.
func TestAcc_ResourceApplication_LocalCharm_NameMismatch(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-local-charm-mismatch")
	dir := t.TempDir()
	// Archive metadata says "actual-charm-name", HCL declares "wrong-name".
	archivePath := buildLocalCharm(t, dir, "actual-charm-name", "v1")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceApplicationLocalCharm(modelName, "app", "wrong-name", archivePath),
				ExpectError: regexp.MustCompile(`Charm Name Mismatch`),
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_ConflictWithChannel verifies that the
// schema attribute validator rejects combining local_path with channel.
func TestAcc_ResourceApplication_LocalCharm_ConflictWithChannel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-local-charm-conflict")
	dir := t.TempDir()
	archivePath := buildLocalCharm(t, dir, "test-charm", "v1")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" { name = %q }
resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "app"
  charm {
    name       = "test-charm"
    local_path = %q
    channel    = "stable"
    base       = "ubuntu@22.04"
  }
}`, modelName, archivePath),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
		},
	})
}

// TestAcc_ResourceApplication_LocalCharm_BaseMismatch verifies that
// ValidateConfig rejects a base that is not listed in the archive's
// manifest.yaml. The test charm declares ubuntu@22.04; requesting
// ubuntu@24.04 should produce an "Unsupported Base" error.
func TestAcc_ResourceApplication_LocalCharm_BaseMismatch(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-local-charm-base")
	dir := t.TempDir()
	// buildLocalCharm produces an archive whose manifest declares ubuntu@22.04.
	archivePath := buildLocalCharm(t, dir, "test-charm", "v1")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" { name = %q }
resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "app"
  charm {
    name       = "test-charm"
    local_path = %q
    base       = "ubuntu@24.04"
  }
}`, modelName, archivePath),
				ExpectError: regexp.MustCompile(`Unsupported Base`),
			},
		},
	})
}

// buildLocalCharm creates a minimal valid .charm archive at <dir>/<name>.charm.
// The archive contains metadata.yaml, manifest.yaml, a dispatch file, and a
// variable-content file so that different calls produce archives with different
// SHA-256 hashes. It returns the path to the created archive.
func buildLocalCharm(t *testing.T, dir, charmName, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}

	archivePath := filepath.Join(dir, charmName+".charm")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("creating charm archive: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	files := map[string]string{
		// metadata.yaml: v2 format with a bases stanza so Juju knows which
		// operating systems the charm supports.
		"metadata.yaml": fmt.Sprintf(
			"name: %s\nsummary: test charm\ndescription: acceptance test charm\nbases:\n  - name: ubuntu\n    channel: \"22.04\"\n",
			charmName,
		),
		// manifest.yaml: must list the supported bases so the controller
		// accepts the charm. An empty list causes "charm does not define any
		// bases".
		"manifest.yaml": "bases:\n  - name: ubuntu\n    channel: \"22.04\"\n    architectures:\n      - amd64\n",
		// dispatch satisfies AddLocalCharm's hasHooksOrDispatch requirement.
		"dispatch": "#!/bin/sh\n",
		// content is the only thing that varies between builds.
		"content": content,
	}
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("adding %q to charm archive: %v", name, err)
		}
		if _, err = fw.Write([]byte(body)); err != nil {
			t.Fatalf("writing %q: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing charm archive: %v", err)
	}
	return archivePath
}

func testAccResourceApplicationLocalCharm(modelName, appName, charmName, archivePath string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = %q

  charm {
    name       = %q
    local_path = %q
    base       = "ubuntu@22.04"
  }
}
`, modelName, appName, charmName, archivePath)
}
