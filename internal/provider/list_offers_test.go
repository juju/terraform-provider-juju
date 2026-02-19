// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccListOffers_query(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-offer-list")
	var offerURL string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceOfferForListing(modelName),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_offer.this"]
					if !ok {
						return fmt.Errorf("not found: juju_offer.this")
					}
					offerURL = rs.Primary.Attributes["id"]
					return nil
				},
			},
			{
				Config: testAccListOffers(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("juju_offer.test", 1),
					querycheck.ExpectResourceKnownValues("juju_offer.test", nil, []querycheck.KnownValueCheck{
						{
							Path: tfjsonpath.New("id"),
							KnownValue: knownvalue.StringFunc(func(v string) error {
								return knownvalue.StringExact(offerURL).CheckValue(v)
							}),
						},
						{
							Path:       tfjsonpath.New("name"),
							KnownValue: knownvalue.StringExact("this"),
						},
						{
							Path: tfjsonpath.New("endpoints"),
							KnownValue: knownvalue.SetExact([]knownvalue.Check{
								knownvalue.StringExact("sink"),
							}),
						},
					}),
					querycheck.ExpectIdentity("juju_offer.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(offerURL).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccResourceOfferForListing(modelName string) string {
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

func testAccListOffers() string {
	return `
list "juju_offer" "test" {
	provider         = juju
	include_resource = true
	config {
		model_uuid       = juju_model.this.uuid
		offer_url        = juju_offer.this.id
	}
}
`
}
