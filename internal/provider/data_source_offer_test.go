// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceOffer(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-offer-test-model")
	// ...-test-[0-9]+ is not a valid offer name, need to remove the dash before numbers
	offerName := fmt.Sprintf("tf-datasource-offer-test%d", acctest.RandInt())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceOffer(modelName, "base = \"ubuntu@22.04\"", offerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("data.juju_offer.this", "name", offerName),
				),
			},
		},
	})
}

func TestAcc_DataSourceOffer_UpgradeProvider(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-offer-test-model")
	// ...-test-[0-9]+ is not a valid offer name, need to remove the dash before numbers
	offerName := fmt.Sprintf("tf-datasource-offer-test%d", acctest.RandInt())

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
				Config: testAccDataSourceOffer(modelName, "series = \"jammy\"", offerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("data.juju_offer.this", "name", offerName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccDataSourceOffer(modelName, "series = \"jammy\"", offerName),
			},
		},
	})
}

func TestAcc_DataSourceOffer_UpgradeProvider_Schema_v0_To_v1(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-offer-test-model")
	// ...-test-[0-9]+ is not a valid offer name, need to remove the dash before numbers
	offerName := fmt.Sprintf("tf-datasource-offer-test%d", acctest.RandInt())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						// This is the version with `endpoint` instead of `endpoints`.
						VersionConstraint: "0.19.0",
						Source:            "juju/juju",
					},
				},
				Config: testAccDataSourceOfferv0(modelName, "series = \"jammy\"", offerName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_offer.this", "model", modelName),
					resource.TestCheckResourceAttr("data.juju_offer.this", "name", offerName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccDataSourceOffer(modelName, "series = \"jammy\"", offerName),
			},
		},
	})
}

func testAccDataSourceOffer(modelName, os, offerName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model_uuid = juju_model.this.uuid
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
	name             = %q
}

data "juju_offer" "this" {
	url = juju_offer.this.url
}
`, modelName, os, offerName)
}

func testAccDataSourceOfferv0(modelName, os, offerName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "this" {
	model_uuid = juju_model.this.uuid
	name  = "this"

	charm {
		name = "juju-qa-dummy-source"
		%s
	}
}

resource "juju_offer" "this" {
	model            = juju_model.this.name
	application_name = juju_application.this.name
	endpoint         = "sink"
	name             = %q
}

data "juju_offer" "this" {
	url = juju_offer.this.url
}
`, modelName, os, offerName)
}
