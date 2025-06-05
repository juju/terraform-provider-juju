// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceOffer(t *testing.T) {
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
				Config: testAccResourceOffer(modelName, "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
				),
			},
			{
				Config: testAccResourceOfferXIntegration(modelName2, destModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.int", "model", destModelName),

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
	model = juju_model.modelone.name
	name  = "appone"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "offerone" {
	model            = juju_model.modelone.name
	application_name = juju_application.appone.name
	endpoints         = ["sink"]
}

resource "juju_model" "modeldest" {
	name = %q
}

resource "juju_application" "apptwo" {
	model = juju_model.modeldest.name
	name = "apptwo"

	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_integration" "int" {
	model = juju_model.modeldest.name

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
				Config: testAccResourceOfferv0(modelName, "series = \"focal\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", expectedResourceOwner(), modelName, "this")),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceOffer(modelName, "series = \"focal\""),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceOffer(modelName, os string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "juju-qa-dummy-source"
		%s
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoints         = ["sink"]
}
`, modelName, os)
}

func testAccResourceOfferv0(modelName, os string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "juju-qa-dummy-source"
		channel = "latest/stable"
		%s
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "sink"
}
`, modelName, os)
}

func TestAcc_ResourceOfferMultipleEndpoints(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with LXD")
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
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName1),
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
	model = juju_model.this.name
	name  = "this"

	charm {
		name = "content-cache-k8s"
		revision = 49
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoints         = ["grafana-dashboard", "metrics-endpoint"]
}

resource "juju_model" "that" {
	name = %q
}

resource "juju_application" "that" {
	model = juju_model.that.name
	name  = "that"
	charm {
	    name = "grafana-agent-k8s"
		revision = 113
    }
}

resource "juju_integration" "offer_db" {
	model = juju_model.that.name
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
	model = juju_model.that.name
	name  = "toc"
	charm {
	    name = "grafana-agent-k8s"
		revision = 113
    }
}

resource "juju_integration" "offer_db_admin" {
	model = juju_model.that.name
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
