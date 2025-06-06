// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceMachine(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachine(modelName, "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "series", "jammy"),
					resource.TestCheckResourceAttr("juju_machine.this", "base", "ubuntu@22.04"),
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

func TestAcc_ResourceMachine_Minimal(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resourceName := "juju_machine.testmachine"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
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

func TestAcc_ResourceMachine_WithPlacement(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resourceName := "juju_machine.this_machine_1"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactoriesNoResourceWait,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineWithPlacement(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckResourceAttr(resourceName, "machine_id", "0/lxd/0"),
					resource.TestCheckResourceAttr(resourceName, "placement", "lxd:0"),
				),
			},
			{
				ImportStateVerify: false,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceMachineBasicMinimal(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "testmachine" {
	model = juju_model.this.name
}
`, modelName)
}

func TestAcc_ResourceMachine_UpgradeProvider(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
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
				Config: testAccResourceMachine(modelName, "series = \"focal\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "series", "focal"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceMachine(modelName, "series = \"focal\""),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceMachine(modelName, operatingSystem string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model = juju_model.this.name
	%s
}
`, modelName, operatingSystem)
}

func TestAcc_ResourceMachine_AddMachine_Edge(t *testing.T) {
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
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineAddMachine(modelName, testAddMachineIP, testSSHPubKeyPath,
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

func TestAcc_ResourceMachine_ConstraintsNormalization(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachine(modelName, "constraints = \"arch=amd64 mem=4G cores=2\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachine(modelName, "constraints = \"cores=2 arch=amd64 mem=4G\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachine(modelName, "constraints = \"mem=4096M cores=2 arch=amd64\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachine(modelName, "constraints = \"mem=4096M cores=4 arch=amd64\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "1"), // Ensure machine is replaced
				),
			},
		},
	})
}

func testAccResourceMachineAddMachine(modelName string, IP string, pubKeyPath string, privKeyPath string) string {
	return fmt.Sprintf(`
resource "juju_model" "this_model" {
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

func testAccResourceMachineWithPlacement(modelName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceMachineWithPlacement",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_machine" "this_machine" {
	name = "manually_provisioned_machine"
	model = juju_model.{{.ModelName}}.name
}

resource "juju_machine" "this_machine_1" {
	model     = juju_model.{{.ModelName}}.name
	name      = "this_machine"
	placement = "lxd:0"
	depends_on = [juju_machine.this_machine]
  }
`, internaltesting.TemplateData{
			"ModelName": modelName,
		})
}
