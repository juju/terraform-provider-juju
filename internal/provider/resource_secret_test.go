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
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

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

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSecretWithoutName(modelName, secretValue, secretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_secret.noname", "model", modelName),
					resource.TestCheckResourceAttr("juju_secret.noname", "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret.noname", "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret.noname", "value.key2", "value2"),
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
					resource.TestCheckResourceAttr("juju_secret."+secretName, "model", modelName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s", modelName, secretName),
				ResourceName:      "juju_secret." + secretName,
			},
		},
	})
}

func TestAcc_ResourceSecret_CheckSecretID(t *testing.T) {
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
		Steps: []resource.TestStep{{
			Config: testAccResourceSecret(modelName, secretName, secretValue, ""),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue(
					"juju_secret."+secretName,
					tfjsonpath.New("secret_id"),
					knownvalue.StringRegexp(regexp.MustCompile("secret:.*")),
				),
			},
		}},
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
					resource.TestCheckResourceAttr("juju_secret."+secretName, "model", modelName),
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
					resource.TestCheckResourceAttr("juju_secret."+secretName, "model", modelName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "name", secretName),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "info", secretInfo),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key1", "value1"),
					resource.TestCheckResourceAttr("juju_secret."+secretName, "value.key2", "value2"),
				),
			},
			{
				Config: testAccResourceSecret(modelName, secretName, secretValueUpdated, updatedSecretInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_secret."+secretName, "model", modelName),
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
  model = juju_model.{{.ModelName}}.name
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
  model = juju_model.{{.ModelName}}.name
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
