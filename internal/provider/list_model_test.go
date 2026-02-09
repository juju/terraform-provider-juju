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

func TestAccListModels_query(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	var modelUUID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_model.model"]
					if !ok {
						return fmt.Errorf("not found: juju_model.model")
					}
					modelUUID = rs.Primary.Attributes["uuid"]
					return nil
				},
			},
			{
				Config: testAccListModels(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_model.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(modelUUID).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListModels() string {
	return `
list "juju_model" "test" {
  provider         = juju
	include_resource = true
}
`
}
