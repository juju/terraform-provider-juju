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
)

func TestAccListSpaces_Query(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-space-list")
	spaceName := "test-space"

	var expectedID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListSpacesSetup(modelName, spaceName),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_space.test"]
					if !ok {
						return fmt.Errorf("not found: juju_space.test")
					}
					expectedID = rs.Primary.ID
					return nil
				},
			},
			{
				Config: testAccListSpaces(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_space.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
				},
			},
			{
				Config: testAccListSpaceExact(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("juju_space.test", 1),
					querycheck.ExpectIdentity("juju_space.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListSpacesSetup(modelName, spaceName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test" {
  name = %q
}

resource "juju_space" "test" {
  model_uuid = juju_model.test.uuid
  name       = %q
}
`, modelName, spaceName)
}

func testAccListSpaces() string {
	return `
list "juju_space" "test" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = juju_model.test.uuid
  }
}
`
}

func testAccListSpaceExact() string {
	return `
list "juju_space" "test" {
  provider         = juju
  include_resource = true

  config {
    model_uuid = juju_model.test.uuid
    name       = juju_space.test.name
  }
}
`
}
