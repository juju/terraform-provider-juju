// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceMachine_Edge(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func TestAcc_DataSourceMachine_UpgradeProvider(t *testing.T) {
	t.Skip("This test currently fails due to the breaking change in the provider schema. " +
		"Remove the skip after the v1 release of the provider.")

	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

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
				Config: testAccDataSourceMachine(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccDataSourceMachine(modelName),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccDataSourceMachine(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}

resource "juju_machine" "machine" {
  model_uuid = juju_model.model.uuid
  name = "machine"
  base = "ubuntu@22.04"
}

data "juju_machine" "machine" {
  model = juju_model.model.name
  machine_id = juju_machine.machine.machine_id
}`, modelName)
}
