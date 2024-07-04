// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// TODO(aflynn): Add add actual usage of the data source to the test. This is
// blocked on the lack of schema for secret access.

func TestAcc_DataSourceSecret(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	t.Parallel()

	version := os.Getenv("JUJU_AGENT_VERSION")
	if version == "" || internaltesting.CompareVersions(version, "3.3.0") < 0 {
		t.Skip("JUJU_AGENT_VERSION is not set or is below 3.3.0")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-secret-test-model")
	// ...-test-[0-9]+ is not a valid secret name, need to remove the dash before numbers
	secretName := fmt.Sprintf("tf-datasource-secret-test%d", acctest.RandInt())
	secretValue := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceSecret(modelName, secretName, secretValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_secret.secret_data_source", "model", modelName),
					resource.TestCheckResourceAttr("data.juju_secret.secret_data_source", "name", secretName),
					resource.TestCheckResourceAttrPair("data.juju_secret.secret_data_source", "secretID", "juju_secret.secret_resource", "secretID"),
				),
			},
		},
	})
}

func testAccDataSourceSecret(modelName, secretName string, secretValue map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_secret" "secret_resource" {
  model = juju_model.{{.ModelName}}.name
  name  = "{{.SecretName}}"
  value =  {
    {{- range $key, $value := .SecretValue }}
    "{{$key}}" = "{{$value}}"
    {{- end }}
  }
}

data "juju_secret" "secret_data_source" {
  name = juju_secret.secret_resource.name
  model = juju_model.{{.ModelName}}.name
}
`, internaltesting.TemplateData{
			"ModelName":   modelName,
			"SecretName":  secretName,
			"SecretValue": secretValue,
		})
}
