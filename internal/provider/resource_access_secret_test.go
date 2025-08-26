// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithAccess(modelName, false, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_access_secret.test_access_secret", "model_uuid"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				Config: testAccResourceSecretWithAccess(modelName, true, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_access_secret.test_access_secret", "model_uuid"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.1", "jul2"),
				),
			},
			{
				Config: testAccResourceSecretWithAccess(modelName, false, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_access_secret.test_access_secret", "model_uuid"),
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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithAccess(modelName, true, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_access_secret.test_access_secret", "model_uuid"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_access_secret.test_access_secret",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					// The import ID for the secret access resource is not the same as the
					// resource ID. It is in the format modelUUID:name, where modelUUID
					// is the UUID of the model and name is the name of the secret.
					rs, ok := s.RootModule().Resources["juju_access_secret.test_access_secret"]
					if !ok {
						return "", fmt.Errorf("resource not found in state: juju_access_secret.test_access_secret")
					}
					id := rs.Primary.Attributes["model_uuid"]
					if id == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:test_secret_name", id), nil
				},
			},
		},
	})
}

func TestAcc_ResourceAccessSecret_UpgradeV0ToV1(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-model")

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
				Config: testAccResourceSecretWithAccess(modelName, true, 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceSecretWithAccess(modelName, true, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model."+modelName, "uuid", "juju_access_secret.test_access_secret", "model_uuid"),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
		},
	})
}

func testAccResourceSecretWithAccess(modelName string, allApplicationAccess bool, resourceVersion int) string {
	// Create the access secret resource based on the version.
	switch resourceVersion {
	case 0, 1:
	default:
		panic(fmt.Sprintf("Unknown resource version %d", resourceVersion))
	}

	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_application" "jul" {
  name  = "jul"
  {{- if eq .ResourceVersion 0 }}
  model = juju_model.{{.ModelName}}.name
  {{- else if eq .ResourceVersion 1 }}
  model_uuid = juju_model.{{.ModelName}}.uuid
  {{- end }}


  charm {
	name     = "ubuntu-lite"
	channel  = "latest/stable"
  }

  units = 1
}

resource "juju_application" "jul2" {
  name  = "jul2"
  {{- if eq .ResourceVersion 0 }}
  model = juju_model.{{.ModelName}}.name
  {{- else if eq .ResourceVersion 1 }}
  model_uuid = juju_model.{{.ModelName}}.uuid
  {{- end }}

  charm {
	name     = "ubuntu-lite"
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
  {{- if eq .ResourceVersion 0 }}
  model = juju_model.{{.ModelName}}.name
  {{- else if eq .ResourceVersion 1 }}
  model_uuid = juju_model.{{.ModelName}}.uuid
  {{- end }}
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
			"ResourceVersion":      resourceVersion,
		})
}

func TestAcc_ResourceAccessSecret_UpgradeProvider_Schema_v0_To_v1(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						// This is the version with `applications` instead of `endpoints`
						VersionConstraint: "0.20.0",
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceSecretWithAccess(modelName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "model", modelName),
					resource.TestCheckResourceAttr("juju_access_secret.test_access_secret", "applications.0", "jul"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceSecretWithAccess_Reorder(modelName, true),
				PlanOnly:                 true,
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceSecretWithAccess_Reorder(modelName, false),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceSecretWithAccess_Reorder(modelName string, reorder bool) string {
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
    name     = "ubuntu-lite"
    channel  = "latest/stable"
  }

  units = 1
}

resource "juju_application" "jul2" {
  name  = "jul2"
  model = juju_model.{{.ModelName}}.name

  charm {
    name     = "ubuntu-lite"
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
  {{- if .Reorder }}
  applications = [
    juju_application.jul.name, juju_application.jul2.name
  ]
  {{- else }}
  applications = [
    juju_application.jul2.name, juju_application.jul.name
  ]
  {{- end }}
  secret_id = juju_secret.test_secret.secret_id
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Reorder":   reorder,
		})
}
