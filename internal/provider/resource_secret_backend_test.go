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

// TestAcc_ResourceSecretBackend verifies the full lifecycle of a secret backend
// resource: create, read (import), update, and delete. It also creates a model
// and a secret to ensure the resource works alongside other resources.
func TestAcc_ResourceSecretBackend(t *testing.T) {
	SkipJAAS(t)
	// The vault backend must be reachable from the Juju controller machines.
	// This test only runs with LXD and requires TEST_VAULT_ADDR to be set
	// to an address the controller can reach (e.g. the LXD bridge IP of the host).
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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create the secret backend, a model, and a secret:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceSecretBackend(backendName, backendType, modelName, secretName, 1, map[string]string{
					"endpoint": vaultEndpoint,
					"token":    "myroot",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", backendName),
					resource.TestCheckResourceAttr(resourceFullName, "backend_type", backendType),
					// Verify the model and secret were also created.
					resource.TestCheckResourceAttrSet("juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key", "value"),
					// Verify the secret is actually stored in the created backend.
					testCheckSecretBackendName(modelName, secretName, backendName),
				),
			},
			// Update the backend config (in-place) by bumping config_wo_version:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceSecretBackend(backendName, backendType, modelName, secretName, 2, map[string]string{
					"endpoint":        vaultEndpoint,
					"token":           "myroot",
					"tls-server-name": "vault.example.com",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceFullName, "name", backendName),
					resource.TestCheckResourceAttr(resourceFullName, "backend_type", backendType),
				),
			},
			// Import the backend to verify Read works:
			{
				ResourceName: resourceFullName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[resourceFullName]
					return rs.Primary.Attributes["name"], nil
				},
				ImportStateVerify: true,
				// config_wo and config_wo_version are write-only and not
				// present in state after import.
				ImportStateVerifyIgnore: []string{"config_wo_version"},
			},
		},
	})
}

func testAccResourceSecretBackend(backendName, backendType, modelName, secretName string, configWOVersion int, config map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceSecretBackend", `
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

resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
  config = {
    secret-backend = juju_secret_backend.{{.BackendName}}.name
  }
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name       = "{{.SecretName}}"
  value = {
    key = "value"
  }
}
`, internaltesting.TemplateData{
		"BackendName":     backendName,
		"BackendType":     backendType,
		"ModelName":       modelName,
		"SecretName":      secretName,
		"ConfigWOVersion": configWOVersion,
		"Config":          config,
	})
}

// testCheckSecretBackendName verifies that the secret with the given name in
// the given model is stored in the specified secret backend. It uses the Juju
// client directly (via TestClient) to list secrets and check the BackendName
// field on the secret's latest revision.
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
