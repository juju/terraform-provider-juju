// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/juju/terraform-provider-juju/internal/juju"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceSecretBackend(t *testing.T) {
	SkipJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	vaultEndpoint := os.Getenv(TestVaultAddrEnvKey)
	if vaultEndpoint == "" {
		t.Skipf("%s is not set, skipping secret backend test", TestVaultAddrEnvKey)
	}

	backendName := acctest.RandomWithPrefix("test-backend")
	backendType := "vault"
	modelName := acctest.RandomWithPrefix("test-model")
	secretName := "test-secret"
	resourceFullName := "juju_secret_backend." + backendName
	modelFullName := "juju_model." + modelName

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create backend, model with secret_backend attr, and a secret.
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceSecretBackend(backendName, backendType, modelName, secretName, "value1", 1, map[string]string{
					"endpoint": vaultEndpoint,
					"token":    "myroot",
				}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", backendName),
					resource.TestCheckResourceAttr(resourceFullName, "backend_type", backendType),
					resource.TestCheckResourceAttr(modelFullName, "secret_backend", backendName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key", "value1"),
					testCheckSecretBackendName(modelName, secretName, backendName),
				),
			},
			// Update backend config (bump config_wo_version) and secret value.
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceSecretBackend(backendName, backendType, modelName, secretName, "value2", 2, map[string]string{
					"endpoint":        vaultEndpoint,
					"token":           "myroot",
					"tls-server-name": "vault.example.com",
				}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceFullName, "name", backendName),
					resource.TestCheckResourceAttr(resourceFullName, "backend_type", backendType),
					testCheckSecretBackendName(modelName, secretName, backendName),
				),
			},
			// Remove secret_backend from model; secret should move to internal.
			{
				Config: testAccResourceSecretBackend(backendName, backendType, modelName, secretName, "value3", 2, map[string]string{
					"endpoint":        vaultEndpoint,
					"token":           "myroot",
					"tls-server-name": "vault.example.com",
				}, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(modelFullName, "secret_backend"),
					testCheckSecretBackendName(modelName, secretName, "internal"),
				),
			},
			// Import the backend.
			{
				ResourceName: resourceFullName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[resourceFullName]
					return rs.Primary.Attributes["name"], nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"config_wo_version"},
			},
		},
	})
}

func TestAcc_ResourceSecretBackend_MigrateFromLegacyConfig(t *testing.T) {
	SkipJAAS(t)
	skipTestIfJujuAgentVersionBelow(t, "3.0.0")
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	vaultEndpoint := os.Getenv(TestVaultAddrEnvKey)
	if vaultEndpoint == "" {
		t.Skipf("%s is not set, skipping secret backend test", TestVaultAddrEnvKey)
	}

	backendName := acctest.RandomWithPrefix("test-backend")
	backendType := "vault"
	modelName := acctest.RandomWithPrefix("test-model")
	secretName := "test-secret"
	resourceFullName := "juju_secret_backend." + backendName
	modelFullName := "juju_model." + modelName

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create with legacy config approach (Juju 3 only).
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceSecretBackendLegacyConfig(backendName, backendType, modelName, secretName, "value1", 1, map[string]string{
					"endpoint": vaultEndpoint,
					"token":    "myroot",
				}, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(modelFullName, "config.secret-backend", backendName),
					resource.TestCheckNoResourceAttr(modelFullName, "secret_backend"),
					testCheckSecretBackendName(modelName, secretName, backendName),
				),
			},
			// Migrate to secret_backend attribute.
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(modelFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceSecretBackendLegacyConfig(backendName, backendType, modelName, secretName, "value2", 1, map[string]string{
					"endpoint": vaultEndpoint,
					"token":    "myroot",
				}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(modelFullName, "secret_backend", backendName),
					resource.TestCheckNoResourceAttr(modelFullName, "config.secret-backend"),
					testCheckSecretBackendName(modelName, secretName, backendName),
				),
			},
		},
	})
}

func testAccResourceSecretBackend(backendName, backendType, modelName, secretName, secretValue string, configWOVersion int, config map[string]string, noSecretBackend bool) string {
	modelBlock := testAccModelBlock(modelName, backendName, noSecretBackend)
	backendBlock := testAccSecretBackendBlock(backendName, backendType, configWOVersion, config)
	secretBlock := testAccSecretBlock(modelName, secretName, secretValue)
	return backendBlock + "\n" + modelBlock + "\n" + secretBlock
}

func testAccResourceSecretBackendLegacyConfig(backendName, backendType, modelName, secretName, secretValue string, configWOVersion int, config map[string]string, useLegacyConfig bool) string {
	modelBlock := testAccModelBlockLegacy(modelName, backendName, useLegacyConfig)
	backendBlock := testAccSecretBackendBlock(backendName, backendType, configWOVersion, config)
	secretBlock := testAccSecretBlock(modelName, secretName, secretValue)
	return backendBlock + "\n" + modelBlock + "\n" + secretBlock
}

func testAccSecretBackendBlock(backendName, backendType string, configWOVersion int, config map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccSecretBackendBlock", `
resource "juju_secret_backend" "{{.BackendName}}" {
  name               = "{{.BackendName}}"
  backend_type       = "{{.BackendType}}"
  config_wo_version  = {{.ConfigWOVersion}}
  config_wo = {
	{{- range $key, $value := .Config }}
    {{$key}} = "{{$value}}"
	{{- end }}
  }
}
`, internaltesting.TemplateData{
		"BackendName":     backendName,
		"BackendType":     backendType,
		"ConfigWOVersion": configWOVersion,
		"Config":          config,
	})
}

func testAccModelBlock(modelName, backendName string, noSecretBackend bool) string {
	return internaltesting.GetStringFromTemplateWithData("testAccModelBlock", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
  {{- if not .NoSecretBackend }}
  secret_backend = juju_secret_backend.{{.BackendName}}.name
  {{- end }}
}
`, internaltesting.TemplateData{
		"ModelName":       modelName,
		"BackendName":     backendName,
		"NoSecretBackend": noSecretBackend,
	})
}

func testAccModelBlockLegacy(modelName, backendName string, useLegacyConfig bool) string {
	return internaltesting.GetStringFromTemplateWithData("testAccModelBlockLegacy", `
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
  {{- if .UseLegacyConfig }}
  config = {
    secret-backend = juju_secret_backend.{{.BackendName}}.name
  }
  {{- else }}
  secret_backend = juju_secret_backend.{{.BackendName}}.name
  {{- end }}
}
`, internaltesting.TemplateData{
		"ModelName":       modelName,
		"BackendName":     backendName,
		"UseLegacyConfig": useLegacyConfig,
	})
}

func testAccSecretBlock(modelName, secretName, secretValue string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccSecretBlock", `
resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name       = "{{.SecretName}}"
  value = {
    key = "{{.SecretValue}}"
  }
}
`, internaltesting.TemplateData{
		"ModelName":   modelName,
		"SecretName":  secretName,
		"SecretValue": secretValue,
	})
}

func testCheckSecretBackendName(modelName, secretName, expectedBackendName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if TestClient == nil {
			return fmt.Errorf("TestClient is not configured")
		}

		modelRes, ok := s.RootModule().Resources["juju_model."+modelName]
		if !ok {
			return fmt.Errorf("model %q not found in state", modelName)
		}
		modelUUID := modelRes.Primary.Attributes["uuid"]

		secrets, err := TestClient.Secrets.ListSecrets(context.Background(), &juju.ListSecretsInput{
			ModelUUID: modelUUID,
			Name:      &secretName,
		})
		if err != nil {
			return fmt.Errorf("unable to list secrets: %w", err)
		}

		if len(secrets) == 0 {
			return fmt.Errorf("no secrets found with name %q in model %q", secretName, modelUUID)
		}

		for _, secret := range secrets {
			if secret.BackendName == expectedBackendName {
				return nil
			}
		}

		var foundBackends []string
		for _, secret := range secrets {
			foundBackends = append(foundBackends, secret.BackendName)
		}
		return fmt.Errorf("secret %q is not stored in backend %q; found backends: %v", secretName, expectedBackendName, foundBackends)
	}
}
