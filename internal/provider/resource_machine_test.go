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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "base", "ubuntu@22.04"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				// We ignore the hostname and wait_for_hostname during ImportVerify in our tests
				// because it is very unlikely it matches the value from the state, since during
				// creation we didn't wait for the hostname to be populated, but it might be the
				// case that during import is populated.
				// This is just an issue that you might face in tests, so it is fine to ignore it.
				ImportStateVerifyIgnore: []string{"wait_for_hostname", "hostname"},
				ResourceName:            "juju_machine.this",
			},
		},
	})
}

func TestAcc_ResourceMachineWaitForHostname(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-wait-for-hostname")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineWaitForHostname(modelName, "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "base", "ubuntu@22.04"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "hostname"),
				),
			},
			{
				ImportState:       true,
				ImportStateVerify: true,
				// We ignore wait_for_hostname because we don't set it during import.
				ImportStateVerifyIgnore: []string{"wait_for_hostname"},
				ResourceName:            "juju_machine.this",
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", resourceName, "model_uuid"),
					resource.TestCheckResourceAttr(resourceName, "machine_id", "0"),
				),
			},
			{
				ImportStateVerify: true,
				// We ignore the hostname and wait_for_hostname during ImportVerify in our tests
				// because it is very unlikely it matches the value from the state, since during
				// creation we didn't wait for the hostname to be populated, but it might be the
				// case that during import it is populated.
				// This is just an issue that you might face in tests, so it is fine to ignore it.
				ImportStateVerifyIgnore: []string{"wait_for_hostname", "hostname"},
				ImportState:             true,
				ResourceName:            resourceName,
			},
		},
	})
}

func TestAcc_ResourceMachine_WithPlacement(t *testing.T) {
	t.Skip("This test is skipped because it is not guaranteed to work on LXD.")
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
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", resourceName, "model_uuid"),
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
	model_uuid = juju_model.this.uuid
}
`, modelName)
}

func TestAcc_ResourceMachine_UpgradeV0ToV1(t *testing.T) {
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
						VersionConstraint: TestProviderPreV1Version,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceMachineV0(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "base", "ubuntu@22.04"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceMachine(modelName),
				PlanOnly:                 true,
			},
		},
	})
}

func TestAcc_ResourceMachine_UpgradeProvider(t *testing.T) {
	t.Skip("This test currently fails due to the breaking change in the provider schema. " +
		"Remove the skip after the v1 release of the provider.")

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
				Config: testAccResourceMachine(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttr("juju_machine.this", "series", "focal"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceMachine(modelName),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceMachine(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model_uuid = juju_model.this.uuid
	base = "ubuntu@22.04"
}
`, modelName)
}

func testAccResourceMachineWithConstraints(modelName, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model_uuid = juju_model.this.uuid
	base = "ubuntu@22.04"
	constraints = %q
}
`, modelName, constraints)
}

func testAccResourceMachineV0(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model = juju_model.this.name
	base = "ubuntu@22.04"
}
`, modelName)
}

func testAccResourceMachineWaitForHostname(modelName, operatingSystem string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_machine" "this" {
	name = "this_machine"
	model_uuid = juju_model.this.uuid
	wait_for_hostname = true
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
					resource.TestCheckResourceAttrPair("juju_model.this_model", "uuid", "juju_machine.this_machine", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "name", "manually_provisioned_machine"),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "machine_id", "0"),
				),
			},
			{
				ImportStateVerify:       true,
				ImportState:             true,
				ImportStateVerifyIgnore: []string{"ssh_address", "public_key_file", "private_key_file", "hostname", "wait_for_hostname"},
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
				Config: testAccResourceMachineWithConstraints(modelName, "arch=amd64 mem=4G cores=2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachineWithConstraints(modelName, "cores=2 arch=amd64 mem=4G"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachineWithConstraints(modelName, "mem=4096M cores=2 arch=amd64"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
					resource.TestCheckResourceAttrSet("juju_machine.this", "machine_id"),
					resource.TestCheckResourceAttr("juju_machine.this", "machine_id", "0"), // Ensure machine is not replaced
				),
			},
			{
				Config: testAccResourceMachineWithConstraints(modelName, "mem=4096M cores=4 arch=amd64"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_machine.this", "model_uuid"),
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
	model_uuid = juju_model.this_model.uuid

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
	model_uuid = juju_model.{{.ModelName}}.uuid
}

resource "juju_machine" "this_machine_1" {
	model_uuid = juju_model.{{.ModelName}}.uuid
	name      = "this_machine"
	placement = "lxd:0"
	depends_on = [juju_machine.this_machine]
  }
`, internaltesting.TemplateData{
			"ModelName": modelName,
		})
}

func TestAcc_ResourceMachine_Annotations(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-annotations")
	machineName := "testmachine"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsMachine(modelName, machineName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.testmachine", "name", machineName),
					resource.TestCheckResourceAttr("juju_machine.testmachine", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsMachine(modelName, machineName, "test", "test-update"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.testmachine", "name", machineName),
					resource.TestCheckResourceAttr("juju_machine.testmachine", "annotations.test", "test-update"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}
  
resource "juju_machine" "testmachine" {
  name = %q
  model_uuid = juju_model.testmodel.uuid
}
`, modelName, machineName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.testmachine", "name", machineName),
					resource.TestCheckNoResourceAttr("juju_machine.testmachine", "annotations.test"),
				),
			},
		},
	})
}

func TestAcc_ResourceMachine_UnsetAnnotations(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine-annotations-unset")
	machineName := "testmachine"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsMachine(modelName, machineName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.testmachine", "name", machineName),
					resource.TestCheckResourceAttr("juju_machine.testmachine", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsMachine(modelName, machineName, "test-another", "test-another"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.testmachine", "name", machineName),
					resource.TestCheckResourceAttr("juju_machine.testmachine", "annotations.test-another", "test-another"),
					resource.TestCheckNoResourceAttr("juju_machine.testmachine", "annotations.test"),
				),
			},
		},
	})
}

func testAccAnnotationsMachine(modelName, machineName string, annotationKey, annotationValue string) string {
	return fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}

resource "juju_machine" "testmachine" {
  name = %q
  model_uuid = juju_model.testmodel.uuid

  
  annotations = {
	%q = %q
  }
}`, modelName, machineName, annotationKey, annotationValue)
}
