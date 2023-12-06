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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOffer(modelName, "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
				),
			},
			{
				Config: testAccResourceOfferXIntegration(modelName2, destModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.int", "model", destModelName),

					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.int", "application.*",
						map[string]string{"name": "apptwo", "endpoint": "db", "offer_url": ""}),

					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.int", "application.*",
						map[string]string{"name": "", "endpoint": "", "offer_url": fmt.Sprintf("%v/%v.%v", "admin",
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
		name = "postgresql"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "offerone" {
	model            = juju_model.modelone.name
	application_name = juju_application.appone.name
	endpoint         = "db"
}

resource "juju_model" "modeldest" {
	name = %q
}

resource "juju_application" "apptwo" {
	model = juju_model.modeldest.name
	name = "apptwo"

	charm {
		name = "hello-juju"
		base = "ubuntu@20.04"
	}
}

resource "juju_integration" "int" {
	model = juju_model.modeldest.name

	application {
		name = juju_application.apptwo.name
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

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceOffer(modelName, "series = \"focal\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_offer.this", "url", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
					resource.TestCheckResourceAttr("juju_offer.this", "id", fmt.Sprintf("%v/%v.%v", "admin", modelName, "this")),
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
		name = "postgresql"
		channel = "latest/stable"
		%s
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "db"
}
`, modelName, os)
}
