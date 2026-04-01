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

func TestAccListMachines_Query(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-list")
	var modelUUID string
	var machineID string
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListMachinesSetup(modelName),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_model.this"]
					if !ok {
						return fmt.Errorf("not found: juju_model.this")
					}
					modelUUID = rs.Primary.Attributes["uuid"]

					machine, ok := s.RootModule().Resources["juju_machine.this"]
					if !ok {
						return fmt.Errorf("not found: juju_machine.this")
					}
					machineID = machine.Primary.Attributes["machine_id"]
					return nil
				},
			},
			{
				Config: testAccListMachines(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_machine.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(newMachineID(modelUUID, machineID, "")).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListMachines() string {
	return ` 
list "juju_machine" "test" {
  provider         = juju
  include_resource = true
  config {
    model_uuid = juju_model.this.uuid
  }
}
`
}

func testAccListMachinesSetup(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name       = "this_machine"
	model_uuid = juju_model.this.uuid
	base       = "ubuntu@24.04"
}
`, modelName)
}
