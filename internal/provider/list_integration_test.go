// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccListIntegrations_QueryAll(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-integration-list")
	var expectedID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListIntegrationsResourceConfig(modelName),
				Check: func(s *terraform.State) error {
					integration, ok := s.RootModule().Resources["juju_integration.test"]
					if !ok {
						return fmt.Errorf("not found: juju_integration.test")
					}
					expectedID = integration.Primary.ID
					modelUUID, _, found := strings.Cut(expectedID, ":")
					if !found || modelUUID == "" {
						return fmt.Errorf("invalid integration id format %q", expectedID)
					}
					return nil
				},
			},
			{
				Config: testAccListIntegrations(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_integration.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
					querycheck.ExpectResourceKnownValues(
						"juju_integration.test",
						queryfilter.ByResourceIdentity(map[string]knownvalue.Check{
							"id": knownvalue.StringFunc(func(actual string) error {
								return knownvalue.StringExact(expectedID).CheckValue(actual)
							}),
						}),
						[]querycheck.KnownValueCheck{
							{
								Path: tfjsonpath.New("model_uuid"),
								KnownValue: knownvalue.StringFunc(func(actual string) error {
									expectedModelUUID, _, found := strings.Cut(expectedID, ":")
									if !found || expectedModelUUID == "" {
										return fmt.Errorf("invalid integration id format %q", expectedID)
									}
									return knownvalue.StringExact(expectedModelUUID).CheckValue(actual)
								}),
							},
						},
					),
				},
			},
			{
				Config: testAccListIntegrationsExact(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_integration.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListIntegrationsResourceConfig(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test" {
	name = %q
}

resource "juju_application" "one" {
	model_uuid = juju_model.test.uuid
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "two" {
	model_uuid = juju_model.test.uuid
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_integration" "test" {
	model_uuid = juju_model.test.uuid

	application {
		name     = juju_application.one.name
		endpoint = "source"
	}

	application {
		name = juju_application.two.name
		endpoint = "sink"
	}
}
`, modelName)
}

func testAccListIntegrations() string {
	return `
list "juju_integration" "test" {
	provider         = juju
	include_resource = true

	config {
		model_uuid = juju_model.test.uuid
	}
}
`
}

func testAccListIntegrationsExact() string {
	return `
list "juju_integration" "test" {
	provider         = juju
	include_resource = true

	config {
		model_uuid       = juju_model.test.uuid
		application_name = juju_application.one.name
	}
}
`
}
