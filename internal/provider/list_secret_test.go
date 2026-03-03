// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAccListSecrets_query(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Skipf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

	modelName := acctest.RandomWithPrefix("tf-test-secret-list")
	secretName := "tf-test-secret"
	secretInfo := "test-info"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	var expectedID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListSecretResourceConfig(modelName, secretName, secretValue, secretInfo),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_secret.test"]
					if !ok {
						return fmt.Errorf("not found: juju_secret.test")
					}
					expectedID = rs.Primary.Attributes["id"]
					return nil
				},
			},
			{
				Config: testAccListSecrets(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_secret.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
					querycheck.ExpectResourceKnownValues(
						"juju_secret.test",
						queryfilter.ByResourceIdentity(map[string]knownvalue.Check{
							"id": knownvalue.StringFunc(func(actual string) error {
								return knownvalue.StringExact(expectedID).CheckValue(actual)
							}),
						}),
						[]querycheck.KnownValueCheck{
							{
								Path:       tfjsonpath.New("name"),
								KnownValue: knownvalue.StringExact(secretName),
							},
							{
								Path: tfjsonpath.New("secret_id"),
								KnownValue: knownvalue.StringFunc(func(actual string) error {
									if actual == "" {
										return fmt.Errorf("secret_id must not be empty")
									}
									return nil
								}),
							},
							{
								Path: tfjsonpath.New("secret_uri"),
								KnownValue: knownvalue.StringFunc(func(actual string) error {
									if actual == "" {
										return fmt.Errorf("secret_uri must not be empty")
									}
									if !strings.HasPrefix(actual, "secret:") {
										return fmt.Errorf("secret_uri must start with secret:, got %q", actual)
									}
									return nil
								}),
							},
							{
								Path:       tfjsonpath.New("info"),
								KnownValue: knownvalue.StringExact(secretInfo),
							},
						},
					),
				},
			},
			{
				Config: testAccListSecretExact(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("juju_secret.test", 1),
					querycheck.ExpectIdentity("juju_secret.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListSecrets() string {
	return `
list "juju_secret" "test" {
  provider         = juju
  include_resource = true

  config {
		model_uuid = juju_model.test.uuid
  }
}
`
}

func testAccListSecretExact() string {
	return `
list "juju_secret" "test" {
  provider         = juju
  include_resource = true

  config {
		model_uuid = juju_model.test.uuid
		name       = juju_secret.test.name
  }
}
`
}

func testAccListSecretResourceConfig(modelName, secretName string, secretValue map[string]string, secretInfo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccListSecretResourceConfig",
		`
resource "juju_model" "test" {
	name = "{{.ModelName}}"
}

resource "juju_secret" "test" {
	model_uuid = juju_model.test.uuid
	name       = "{{.SecretName}}"
	value = {
		{{- range $key, $value := .SecretValue }}
		"{{$key}}" = "{{$value}}"
		{{- end }}
	}
	{{- if ne .SecretInfo "" }}
	info = "{{.SecretInfo}}"
	{{- end }}
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretName":  secretName,
			"SecretValue": secretValue,
			"SecretInfo":  secretInfo,
		})
}
