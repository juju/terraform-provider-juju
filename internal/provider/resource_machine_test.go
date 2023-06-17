package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceMachine_Basic(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-machine")
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineBasic(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this", "name", "this_machine"),
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

func testAccResourceMachineBasic(modelName string) string {
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

func TestAcc_ResourceMachine_AddMachine(t *testing.T) {
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
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceMachineAddMachine(modelName, testAddMachineIP, testSSHPubKeyPath, testSSHPrivKeyPath),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_machine.this_machine", "model", modelName),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "name", "manually_provisioned_machine"),
					resource.TestCheckResourceAttr("juju_machine.this_machine", "machine_id", "0"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_machine.this_machine",
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
