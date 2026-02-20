// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceSecret_CreateWithoutName(t *testing.T) {
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

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
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

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
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

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
	agentVersion := os.Getenv(TestJujuAgentVersion)
	if agentVersion == "" {
		t.Errorf("%s is not set", TestJujuAgentVersion)
	} else if internaltesting.CompareVersions(agentVersion, "3.3.0") < 0 {
		t.Skipf("%s is not set or is below 3.3.0", TestJujuAgentVersion)
	}

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
