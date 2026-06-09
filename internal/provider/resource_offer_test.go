// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/names/v6"

	"github.com/juju/terraform-provider-juju/internal/juju"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceOffer(t *testing.T) {
	t.Skip(`See https://github.com/juju/juju/issues/22238.
	
	Removing test step 1 i.e. config 'testAccResourceOffer' makes the test pass.
	Removing test step 2 i.e. config 'testAccResourceOfferXIntegration' makes the test pass.
	
	Running both steps together causes the test to fail. Keeping this test as-is but marking
	as skipped to allow for investigation into the underlying issue.`)

	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-offer")
	modelName2 := acctest.RandomWithPrefix("tf-test-offer")
	destModelName := acctest.RandomWithPrefix("tf-test-offer-dest")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOffer(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_offer.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
				),
			},
			{
				Config: testAccResourceOfferXIntegration(modelName2, destModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.modeldest", "uuid", "juju_integration.int", "model_uuid"),

					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.int", "application.*",
						map[string]string{"name": "apptwo", "endpoint": "source", "offer_url": ""}),

					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.int", "application.*",
						map[string]string{"name": "", "endpoint": "", "offer_url": fmt.Sprintf("%v/%v.%v", expectedResourceOwner(),
							modelName2, "appone")}),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_offer.offerone",
			},
		},
	})
}

func testAccResourceOfferXIntegration(srcModelName string, destModelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "modelone" {
	name = %q
}

resource "juju_application" "appone" {
	model_uuid = juju_model.modelone.uuid
	name  = "appone"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "offerone" {
	model_uuid = juju_model.modelone.uuid
	application_name = juju_application.appone.name
	endpoints         = ["sink"]
}

resource "juju_model" "modeldest" {
	name = %q
}

resource "juju_application" "apptwo" {
	model_uuid = juju_model.modeldest.uuid
	name = "apptwo"

	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_integration" "int" {
	model_uuid = juju_model.modeldest.uuid

	application {
		name = juju_application.apptwo.name
		endpoint = "source"
	}

	application {
		offer_url = juju_offer.offerone.url
	}
}
`, srcModelName, destModelName)
}

func TestAcc_ResourceOffer_UpgradeProvider(t *testing.T) {
	// This skip is temporary until we have a stable version of the provider that supports
	// Juju 4.0.0 and above, at which point we can re-enable it.
	SkipAgainstJuju4(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-offer")

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
				Config: testAccResourceOffer(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_offer.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceOffer(modelName),
			},
		},
	})
}

func testAccResourceOffer(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model_uuid = juju_model.this.uuid
	name  = "this"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "this" {
	model_uuid       = juju_model.this.uuid
	application_name = juju_application.this.name
	endpoints        = ["sink"]
}
`, modelName)
}

func TestAcc_ResourceOfferMultipleEndpoints(t *testing.T) {
	SkipAgainstJuju4WithReason(t, "See https://github.com/juju/juju/issues/22213")
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with Microk8s")
	}
	modelName1 := acctest.RandomWithPrefix("tf-test-offer")
	modelName2 := acctest.RandomWithPrefix("tf-test-offer")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOfferMultipleEndpoints(modelName1, modelName2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_offer.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_offer.this", "endpoints.0", "grafana-dashboard"),
					resource.TestCheckResourceAttr("juju_offer.this", "endpoints.1", "metrics-endpoint"),
					resource.TestCheckResourceAttr("juju_offer.this", "endpoints.#", "2"),
				),
			},
		},
	})
}

func testAccResourceOfferMultipleEndpoints(modelName1, modelName2 string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model_uuid = juju_model.this.uuid
	name  = "this"

	charm {
		name = "content-cache-k8s"
		revision = 49
		channel = "latest/stable"
	}
}

resource "juju_offer" "this" {
	model_uuid       = juju_model.this.uuid
	application_name = juju_application.this.name
	endpoints        = ["grafana-dashboard", "metrics-endpoint"]
}

resource "juju_model" "that" {
	name = %q
}

resource "juju_application" "that" {
	model_uuid = juju_model.that.uuid
	name  = "that"
	charm {
	    name = "grafana-agent-k8s"
		revision = 164
		channel = "1/stable"
    }
}

resource "juju_integration" "offer_db" {
	model_uuid = juju_model.that.uuid
	application {
		name     = juju_application.that.name
		endpoint = "metrics-endpoint"
	}
	application {
		offer_url = juju_offer.this.url
		endpoint = "metrics-endpoint"
	}
}

resource "juju_application" "toc" {
	model_uuid = juju_model.that.uuid
	name  = "toc"
	charm {
	    name = "grafana-agent-k8s"
		revision = 164
		channel = "1/stable"
    }
}

resource "juju_integration" "offer_db_admin" {
	model_uuid = juju_model.that.uuid
	application {
		name     = juju_application.toc.name
		endpoint = "grafana-dashboards-consumer"
	}
	application {
		offer_url = juju_offer.this.url
		endpoint = "grafana-dashboard"
	}
}
`, modelName1, modelName2)
}

func TestAcc_ResourceOfferFuzzyName(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-offer-fuzzy")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// First apply: create only "haproxy-two".
				Config: testAccResourceOfferFuzzyNameStep1(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.haproxy_two", "name", "haproxy-two"),
				),
			},
			{
				// Second apply: add "haproxy" where its name is a substring of "haproxy-two".
				// Historically this could fail during read-after-create due to fuzzy offer-name matching.
				Config: testAccResourceOfferFuzzyNameStep2(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.haproxy", "name", "haproxy"),
					resource.TestCheckResourceAttr("juju_offer.haproxy_two", "name", "haproxy-two"),
				),
			},
		},
	})
}

func testAccResourceOfferFuzzyNameStep1(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "haproxy" {
	model_uuid = juju_model.this.uuid
	name  = "haproxy"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "haproxy_two" {
	model_uuid = juju_model.this.uuid
	name  = "haproxy-two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "haproxy_two" {
	model_uuid       = juju_model.this.uuid
	name             = "haproxy-two"
	application_name = juju_application.haproxy_two.name
	endpoints        = ["sink"]
}
`, modelName)
}

func testAccResourceOfferFuzzyNameStep2(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "haproxy" {
	model_uuid = juju_model.this.uuid
	name  = "haproxy"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "haproxy_two" {
	model_uuid = juju_model.this.uuid
	name  = "haproxy-two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "haproxy" {
	model_uuid       = juju_model.this.uuid
	name             = "haproxy"
	application_name = juju_application.haproxy.name
	endpoints        = ["sink"]
}

resource "juju_offer" "haproxy_two" {
	model_uuid       = juju_model.this.uuid
	name             = "haproxy-two"
	application_name = juju_application.haproxy_two.name
	endpoints        = ["sink"]
}
`, modelName)
}

func TestAcc_ResourceOfferTwoOffersSameApplication(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-offer-same-app")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOfferTwoOffersSameApplication(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.haproxy", "name", "haproxy"),
					resource.TestCheckResourceAttr("juju_offer.haproxy_two", "name", "haproxy-two"),
					resource.TestCheckResourceAttrPair("juju_offer.haproxy", "application_name", "juju_offer.haproxy_two", "application_name"),
				),
			},
		},
	})
}

func testAccResourceOfferTwoOffersSameApplication(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "haproxy" {
	model_uuid = juju_model.this.uuid
	name  = "haproxy"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "haproxy" {
	model_uuid       = juju_model.this.uuid
	name             = "haproxy"
	application_name = juju_application.haproxy.name
	endpoints        = ["sink"]
}

resource "juju_offer" "haproxy_two" {
	model_uuid       = juju_model.this.uuid
	name             = "haproxy-two"
	application_name = juju_application.haproxy.name
	endpoints        = ["sink"]
}
`, modelName)
}

// TestAcc_ResourceOffer_DeleteTimeout simulates a practitioner deleting an offer.
// Unbeknownst to them, someone else has created an integration consuming this offer
// out of band with Terraform.
// After investigating whether it's appropriate, the practitioner decies to allow
// force destroy and try again.
func TestAcc_ResourceOffer_DeleteTimeout(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	srcModelName := acctest.RandomWithPrefix("tf-test-offer-src-delete")
	dstModelName := acctest.RandomWithPrefix("tf-test-offer-dst-delete")

	var dstModelUUID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Setup practitioner's src model with offer
				Config: testAccResourceOfferToDelete(srcModelName, internaltesting.TemplateData{
					"AllowForceDestroy": false,
					"IncludeOffer":      true,
				}),
				// And rogue dst model with integration
				Check: func(s *terraform.State) error {
					offer, ok := s.RootModule().Resources["juju_offer.this"]
					if !ok {
						return fmt.Errorf("not found: juju_offer.this")
					}
					offerURL := offer.Primary.Attributes["url"]
					if offerURL == "" {
						return fmt.Errorf("missing juju_offer.this.url")
					}

					// Update closure capture and return any error
					var err error
					dstModelUUID, err = createDstModel(dstModelName, offerURL)
					return err
				},
			},
			{
				// Drop the offer. Destroy should time out with an active connection error
				Config: testAccResourceOfferToDelete(srcModelName, internaltesting.TemplateData{
					"AllowForceDestroy": false,
					"IncludeOffer":      false,
				}),
				ExpectError: regexp.MustCompile(`(?s)still\s+has\s+connection\(s\)\s+after\s+timeout`),
			},
			{
				// Restore the offer and store allow_force_destroy=true in state
				Config: testAccResourceOfferToDelete(srcModelName, internaltesting.TemplateData{
					"AllowForceDestroy": true,
					"IncludeOffer":      true,
				}),
			},
			{
				// Drop the offer. Force destroy should happen on timeout
				Config: testAccResourceOfferToDelete(srcModelName, internaltesting.TemplateData{
					"AllowForceDestroy": true,
					"IncludeOffer":      false,
				}),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckOfferRemoved,
					// Clean up dst model
					func(s *terraform.State) error {
						return destroyDstModel(dstModelUUID)
					},
				),
			},
		},
	})
}

func testAccResourceOfferToDelete(srcModelName string, data internaltesting.TemplateData) string {
	data["SrcModelName"] = srcModelName
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceOfferToDelete",
		`
resource "juju_model" "src" {
  name = "{{.SrcModelName}}"
}

resource "juju_application" "src" {
  model_uuid = juju_model.src.uuid
  name       = "src"
  charm {
    name = "juju-qa-dummy-source"
    base = "ubuntu@22.04"
  }
  config = {
    token = "abc"
  }
}

{{ if .IncludeOffer }}
resource "juju_offer" "this" {
  model_uuid          = juju_model.src.uuid
  application_name    = juju_application.src.name
  endpoints           = ["sink"]
  allow_force_destroy = {{.AllowForceDestroy}}
  timeouts {
    delete = "10s"
  }
}
{{ end }}
`,
		data,
	)
}

func testAccCheckOfferRemoved(s *terraform.State) error {
	// juju_offer.this has already been destroyed so it is absent from state.
	// Derive the offer URL from the src model that is still present.
	srcModel, ok := s.RootModule().Resources["juju_model.src"]
	if !ok {
		return fmt.Errorf("not found: juju_model.src")
	}
	offerURL := fmt.Sprintf("%s/%s.src", expectedResourceOwner(), srcModel.Primary.Attributes["name"])

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	for {
		_, err := TestClient.Offers.ReadOffer(ctx, &juju.ReadOfferInput{
			OfferURL: offerURL,
		})
		if err != nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("offer %q still exists after force destroy and timeout", offerURL)
		case <-time.After(1 * time.Second):
		}
	}
}

func createDstModel(dstModelName string, offerURL string) (string, error) {
	ctx := context.Background()

	modelResp, err := TestClient.Models.CreateModel(ctx, juju.CreateModelInput{
		Name: dstModelName,
	})
	if err != nil {
		return "", fmt.Errorf("creating dst model: %w", err)
	}
	dstModelUUID := modelResp.UUID

	_, err = TestClient.Applications.CreateApplication(ctx, &juju.CreateApplicationInput{
		ApplicationName: "dst",
		ModelUUID:       dstModelUUID,
		CharmName:       "juju-qa-dummy-sink",
		CharmChannel:    "stable",
		CharmRevision:   juju.UnspecifiedRevision,
		CharmBase:       "ubuntu@22.04",
	})
	if err != nil {
		return "", fmt.Errorf("creating dst application: %w", err)
	}

	_, err = TestClient.Offers.ConsumeRemoteOffer(ctx, &juju.ConsumeRemoteOfferInput{
		ModelUUID: dstModelUUID,
		OfferURL:  offerURL,
	})
	if err != nil {
		return "", fmt.Errorf("consuming remote offer: %w", err)
	}

	_, err = TestClient.Integrations.CreateIntegration(ctx, &juju.IntegrationInput{
		ModelUUID: dstModelUUID,
		Apps:      []string{"dst"},
		Endpoints: []string{"dst:source", "src"},
	})
	if err != nil {
		return "", fmt.Errorf("creating integration: %w", err)
	}

	return dstModelUUID, nil
}

func destroyDstModel(modelUUID string) error {
	ctx := context.Background()
	conn, err := TestClient.Models.GetConnection(ctx, nil)
	if err != nil {
		return fmt.Errorf("getting connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)
	tag := names.NewModelTag(modelUUID)
	destroyStorage := false
	forceDestroy := true
	maxWait := 1 * time.Second
	timeout := 10 * time.Second

	if err := client.DestroyModel(ctx, tag, &destroyStorage, &forceDestroy, &maxWait, &timeout); err != nil {
		log.Printf("[WARN] destroyDstModel: ignoring failure from force-destroy of model %s: %v", modelUUID, err)
	}
	return nil
}
