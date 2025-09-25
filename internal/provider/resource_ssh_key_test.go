// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceSSHKey(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere`

	sshKey2 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSSHKey(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
			// we update the key
			{
				Config: testAccResourceSSHKey(modelName, sshKey2),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey2)),
			},
		},
	})
}

func TestAcc_ResourceSSHKey_ColonInKeyIdentifier(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere:ssh-rsa`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSSHKey(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
		},
	})
}

func TestAcc_ResourceSSHKey_WithoutComment(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs=`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSSHKey(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
		},
	})
}

func TestAcc_AddMultipleSSHKeys(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKeys := []string{
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere`,
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi8nKojuW jimmy@somewhere`,
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi7nKojuW jimmy@somewhere`,
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi6nKojuW jimmy@somewhere`,
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi5nKojuW jimmy@somewhere`,
		`ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi4nKojuW jimmy@somewhere`,
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccMultipleSSHKeys(modelName, sshKeys),
			},
			{
				Config: testAccMultipleSSHKeys(modelName, sshKeys[:4]),
			},
		},
	})
}

func TestAcc_ResourceSSHKey_ED25519(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere`

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSSHKey(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
		},
	})
}

func TestAcc_ResourceSSHKey_UpgradeProvider(t *testing.T) {
	t.Skip("This test currently fails due to the breaking change in the provider schema. " +
		"Remove the skip after the v1 release of the provider.")

	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere`

	sshKey2 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere`

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
				Config: testAccResourceSSHKey(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_ssh_key.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceSSHKey(modelName, sshKey2),
			},
		},
	})
}

func TestAcc_ResourceSSHKey_UpgradeV0ToV1(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-sshkey")
	sshKey1 := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABgQC1I8QDP79MaHEIAlfh933zqcE8LyUt9doytF3YySBUDWippk8MAaKAJJtNb+Qsi+Kx/RsSY02VxMy9xRTp9d/Vr+U5BctKqhqf3ZkJdTIcy+z4hYpFS8A4bECJFHOnKIekIHD9glHkqzS5Vm6E4g/KMNkKylHKlDXOafhNZAiJ1ynxaZIuedrceFJNC47HnocQEtusPKpR09HGXXYhKMEubgF5tsTO4ks6pplMPvbdjxYcVOg4Wv0N/LJ4ffAucG9edMcKOTnKqZycqqZPE6KsTpSZMJi2Kl3mBrJE7JbR1YMlNwG6NlUIdIqVoTLZgLsTEkHqWi6OExykbVTqFuoWJJY2BmRAcP9T3FdLYbqcajfWshwvPM2AmYb8V3zBvzEKL1rpvG26fd3kGhk3Vu07qAUhHLMi3P0McEky4cLiEWgI7UyHFLI2yMRZgz23UUtxhRSkvCJagRlVG/s4yoylzBQJir8G3qmb36WjBXxpqAXhfLxw05EQI1JGV3ReYOs= jimmy@somewhere`

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
				Config: testAccResourceSSHKeyV0(modelName, sshKey1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_ssh_key.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_ssh_key.this", "payload", sshKey1)),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceSSHKey(modelName, sshKey1),
			},
		},
	})
}

func testAccResourceSSHKeyV0(modelName string, sshKey string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_ssh_key" "this" {
	model   = juju_model.this.name
	payload = %q
}
`, modelName, sshKey)
}

func testAccResourceSSHKey(modelName string, sshKey string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_ssh_key" "this" {
	model_uuid = juju_model.this.uuid
	payload    = %q
}
`, modelName, sshKey)
}

func testAccMultipleSSHKeys(modelName string, sshKeys []string) string {
	quotedKeys := make([]string, len(sshKeys))
	for i, k := range sshKeys {
		quotedKeys[i] = fmt.Sprintf("%q", k)
	}
	return fmt.Sprintf(`
	resource "juju_model" "this" {
		name = %q
	}

	variable "ssh_keys" {
		type = list(string)
		default = [%s]
	}

	resource "juju_ssh_key" "this" {
		count       = length(var.ssh_keys)
		model_uuid  = juju_model.this.uuid
		payload     = var.ssh_keys[count.index]
	}
	`, modelName, strings.Join(quotedKeys, ","))
}
