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
					resource.TestCheckResourceAttrPair("juju_model.model", "uuid", "data.juju_machine.machine", "model_uuid"),
				),
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
  model_uuid = juju_model.model.uuid
  machine_id = juju_machine.machine.machine_id
}`, modelName)
}

func TestAcc_ResourceMachineWaitForIPAddresses(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-wait-for-ip")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineWaitForIPAddresses(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("data.juju_machine.this", "wait_for_ip_addresses.#", "1"),
					resource.TestCheckResourceAttr("data.juju_machine.this", "ip_addresses.#", "1"),
					resource.TestCheckResourceAttrSet("data.juju_machine.this", "ip_addresses.0"),
				),
			},
		},
	})
}

func testAccResourceMachineWaitForIPAddresses(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model_uuid = juju_model.this.uuid
	base = "ubuntu@22.04"
}

data "juju_machine" "this" {
	model_uuid = juju_model.this.uuid
	machine_id = juju_machine.this.machine_id
	wait_for_ip_addresses = ["any"]
}`, modelName)
}
