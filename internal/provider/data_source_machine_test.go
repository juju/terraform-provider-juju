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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine(modelName, "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func TestAcc_DataSourceMachine_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-machine-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceMachine(modelName, "series = \"jammy\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_machine.machine", "model", modelName),
				),
			},
		},
	})
}

func testAccDataSourceMachine(modelName, os string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}

resource "juju_machine" "machine" {
  model = juju_model.model.name
  name = "machine"
  %s
}

data "juju_machine" "machine" {
  model = juju_model.model.name
  machine_id = juju_machine.machine.machine_id
}`, modelName, os)
}
