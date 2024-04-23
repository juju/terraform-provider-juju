// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// TestAcc_ResourceAccessSecret_GrantRevoke tests the creation of a secret access resource. This is a contrived test as
// the applications used don't actually require a user secret.
// TODO(anvial): Add a test that uses a secret that is actually required by the application.
func TestAcc_ResourceAccessSecret_GrantRevoke(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithAccess(modelName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				Config: testAccResourceSecretWithAccess(modelName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.1", "jul2"),
				),
			},
			{
				Config: testAccResourceSecretWithAccess(modelName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
		},
	})
}

func TestAcc_ResourceAccessSecret_Import(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithAccess(modelName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:test_secret_name", modelName),
				ResourceName:      "juju_access_secret.test_access_secret",
			},
		},
	})
}

func testAccResourceSecretWithAccess(modelName string, allApplicationAccess bool) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "jul" {
  name  = "jul"
  model = juju_model.{{.ModelName}}.name

  charm {
    name     = "jameinel-ubuntu-lite"
    channel  = "latest/stable"
  }

  units = 1
}

resource "juju_application" "jul2" {
  name  = "jul2"
  model = juju_model.{{.ModelName}}.name

  charm {
    name     = "jameinel-ubuntu-lite"
    channel  = "latest/stable"
  }

  units = 1
}

resource "juju_secret" "test_secret" {
  model = juju_model.{{.ModelName}}.name
  name  = "test_secret_name"
  value = {
    key1 = "value1"
    key2 = "value2"
  }
  info  = "This is my secret"
}

resource "juju_access_secret" "test_access_secret" {
  model = juju_model.{{.ModelName}}.name
  {{- if .AllApplicationAccess }}
  applications = [
    juju_application.jul.name, juju_application.jul2.name
  ]
  {{- else }}
  applications = [
    juju_application.jul.name
  ]
  {{- end }}
  secret_id = juju_secret.test_secret.secret_id
}
`, internaltesting.TemplateData{
			"ModelName":            modelName,
			"AllApplicationAccess": allApplicationAccess,
		})
}
