// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceSecret_CreateWithoutName(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretInfo := "test-info"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	secretURIregexp := regexp.MustCompile("^secret:.+")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithoutName(modelName, secretValue, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret.noname", "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret.noname", "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret.noname", "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret.noname", "value.key2", "value2"),
					resource.TestMatchResourceAttr("juju_secret.noname", "secret_uri", secretURIregexp),
				),
			},
		},
	})
}

func TestAcc_ResourceSecret_CreateWithInfo(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretInfo := "test-info"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecret(modelName, secretName, secretValue, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["juju_secret."+secretName]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					id := rs.Primary.Attributes["model_uuid"]
					if id == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:%s", id, secretName), nil
				},
				ResourceName: "juju_secret." + secretName,
			},
		},
	})
}

func TestAcc_ResourceSecret_CreateWithNoInfo(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecret(modelName, secretName, secretValue, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
				),
			},
		},
	})
}

func TestAcc_ResourceSecret_Update(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretInfo := "test-info"

	updatedSecretInfo := "updated-test-info"

	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	secretValueUpdated := map[string]string{
		"key1": "value1",
		"key2": "newValue2",
		"key3": "value3",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecret(modelName, secretName, secretValue, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
				),
			},
			{
				Config: testAccResourceSecret(modelName, secretName, secretValueUpdated, updatedSecretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", updatedSecretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "newValue2"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key3", "value3"),
				),
			},
		},
	})
}

func TestAcc_ResourceSecret_CreateUpdateWriteOnlyValue(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretInfo := "test-info"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	secretValueUpdated := map[string]string{
		"key1": "value1",
		"key2": "newValue2",
		"key3": "value3",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		Steps: []resource.TestStep{
			{
				// Create the secret using a write-only value.
				Config: testAccResourceSecretWriteOnly(modelName, secretName, secretValue, 1, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value_wo_version", "1"),
					// The write-only value must never be stored in state.
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value.%"),
				),
			}, {
				// Bumping value_wo_version triggers an update of the write-only value.
				Config: testAccResourceSecretWriteOnly(modelName, secretName, secretValueUpdated, 2, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value_wo_version", "2"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value.%"),
				),
			},
		},
	})
}

func TestAcc_ResourceSecret_MigrateValueToWriteOnlyAndBack(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretInfo := "test-info"
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	secretValueWriteOnly := map[string]string{
		"key1": "value1",
		"key2": "newValue2",
		"key3": "value3",
	}
	secretValueFinal := map[string]string{
		"key1": "value1",
		"key2": "finalValue2",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		Steps: []resource.TestStep{
			{
				// Create the secret using a regular value.
				Config: testAccResourceSecret(modelName, secretName, secretValue, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_secret."+secretName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo"),
				),
			}, {
				// Migrate from value to a write-only value.
				Config: testAccResourceSecretWriteOnly(modelName, secretName, secretValueWriteOnly, 1, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value_wo_version", "1"),
					// The write-only value must never be stored in state.
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value.%"),
				),
			}, {
				// Migrate back from a write-only value to a regular value.
				Config: testAccResourceSecret(modelName, secretName, secretValueFinal, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "finalValue2"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value.key3"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo"),
					resource.TestCheckNoResourceAttr("juju_secret."+secretName, "value_wo_version"),
				),
			},
		},
	})
}

func TestAcc_ResourceSecret_WriteOnlyConfigValidation(t *testing.T) {
	skipTestIfSecretsNotSupported(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	secretName := "tf-test-secret"
	secretValue := map[string]string{
		"key1": "value1",
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.11.0"))),
		},
		Steps: []resource.TestStep{
			{
				// Both value and value_wo set -> ExactlyOneOf violation.
				Config:      testAccResourceSecretBothValues(modelName, secretName, secretValue),
				ExpectError: regexp.MustCompile(`(?s)Exactly one of these attributes must be configured: \[value,value_wo\]`),
			},
			{
				// Neither value nor value_wo set -> ExactlyOneOf violation.
				Config:      testAccResourceSecretNoValues(modelName, secretName),
				ExpectError: regexp.MustCompile(`(?s)Exactly one of these attributes must be configured: \[value,value_wo\]`),
			},
			{
				// value_wo without value_wo_version -> RequiredTogether violation.
				Config:      testAccResourceSecretWriteOnlyNoVersion(modelName, secretName, secretValue),
				ExpectError: regexp.MustCompile(`(?s)These attributes must be configured together: \[value_wo,value_wo_version\]`),
			},
		},
	})
}

func testAccResourceSecret(modelName, secretName string, secretValue map[string]string, secretInfo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name  = "{{.SecretName}}"
  value =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
  {{- if ne .SecretInfo "" }}
  info  = "{{.SecretInfo}}"
  {{- end }}
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretName":  secretName,
			"SecretValue": secretValue,
			"SecretInfo":  secretInfo,
		})
}

func testAccResourceSecretWithoutName(modelName string, secretValue map[string]string, secretInfo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "noname" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  value =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
  {{- if ne .SecretInfo "" }}
  info  = "{{.SecretInfo}}"
  {{- end }}
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretValue": secretValue,
			"SecretInfo":  secretInfo,
		})
}

func testAccResourceSecretWriteOnly(modelName, secretName string, secretValue map[string]string, secretValueVersion int, secretInfo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecretWriteOnly",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name  = "{{.SecretName}}"
  value_wo =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
  value_wo_version = {{.SecretValueVersion}}
  {{- if ne .SecretInfo "" }}
  info  = "{{.SecretInfo}}"
  {{- end }}
}
`, internaltesting.TemplateData{
			"ModelName":          modelName,
			"SecretName":         secretName,
			"SecretValue":        secretValue,
			"SecretValueVersion": secretValueVersion,
			"SecretInfo":         secretInfo,
		})
}

func testAccResourceSecretBothValues(modelName, secretName string, secretValue map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecretBothValues",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name  = "{{.SecretName}}"
  value =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
  value_wo =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
  value_wo_version = 1
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretName":  secretName,
			"SecretValue": secretValue,
		})
}

func testAccResourceSecretNoValues(modelName, secretName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecretNoValues",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name  = "{{.SecretName}}"
}
`, internaltesting.TemplateData{
			"ModelName":  modelName,
			"SecretName": secretName,
		})
}

func testAccResourceSecretWriteOnlyNoVersion(modelName, secretName string, secretValue map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecretWriteOnlyNoVersion",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "{{.SecretName}}" {
  model_uuid = juju_model.{{.ModelName}}.uuid
  name  = "{{.SecretName}}"
  value_wo =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretName":  secretName,
			"SecretValue": secretValue,
		})
}
