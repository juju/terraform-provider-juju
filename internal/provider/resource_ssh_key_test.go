package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceSSHKey_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere
`
	sshKey2 := `
ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBxxpqAXhfLxw05EQI1JGV3ReYOs= johnny@elsewhere
`
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSSHKey(modelName, sshKey1, sshKey2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_ssh_key.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "key.#]", "2")),
			},
		},
	})
}

func testAccResourceSSHKey(modelName string, sshKey1 string, sshKey2 string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_ssh_key" "this" {
	model = juju_model.this.name
	key = {
		payload= %q
	}
	key = {
		payload= %q
	}
}
`, modelName, sshKey1, sshKey2)
}
