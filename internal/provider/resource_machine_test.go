// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceMachine_sdk2_framework_migrate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineBasicMigrate(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "series", "focal"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_machine.this",
			},
		},
	})
}

func testAccResourceMachineBasicMigrate(modelName string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this" {
    provider = oldjuju
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model = juju_model.this.name
	series = "focal"
}
`, modelName)
}

func TestAcc_ResourceMachine_Minimal(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resourceName := "juju_machine.testmachine"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineBasicMinimal(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckResourceAttr(resourceName, "machine_id", "0"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceMachineBasicMinimal(modelName string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this" {
    provider = oldjuju
	name = %q
}

resource "juju_machine" "testmachine" {
	model = juju_model.this.name
}
`, modelName)
}

func TestAcc_ResourceMachine_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
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
				Config: testAccResourceMachineStable(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "series", "focal"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_machine.this",
			},
		},
	})
}

func testAccResourceMachineStable(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model = juju_model.this.name
	series = "focal"
}
`, modelName)
}

func TestAcc_ResourceMachine_AddMachine_sdk2_framework_migrate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	if testAddMachineIP == "" {
		t.Skipf("environment variable %v not setup or invalid for running test", TestMachineIPEnvKey)
	}
	if testSSHPubKeyPath == "" || testSSHPrivKeyPath == "" {
		t.Skipf("expected environment variables for ssh keys to be set : %v, %v",
			TestSSHPublicKeyFileEnvKey, TestSSHPrivateKeyFileEnvKey)
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-ssh-address")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineAddMachineMigrate(modelName, testAddMachineIP, testSSHPubKeyPath,
					testSSHPrivKeyPath),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this_machine", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "name", "manually_provisioned_machine"),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "machine_id", "0"),
				),
			},
			{
				ImportStateVerify:       true,
				ImportState:             true,
				ImportStateVerifyIgnore: []string{"ssh_address", "public_key_file", "private_key_file"},
				ResourceName:            "juju_machine.this_machine",
			},
		},
	})
}

func testAccResourceMachineAddMachineMigrate(modelName string, IP string, pubKeyPath string, privKeyPath string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this_model" {
    provider = oldjuju
	name = %q
}

resource "juju_machine" "this_machine" {
	name = "manually_provisioned_machine"
	model = juju_model.this_model.name

	ssh_address = "ubuntu@%v"
    public_key_file = %q
    private_key_file = %q
}
`, modelName, IP, pubKeyPath, privKeyPath)
}
