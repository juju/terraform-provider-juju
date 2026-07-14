// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// TestAcc_DataSourceAction tests the juju_action data source. It runs an
// action via the juju_action resource, then references that action's ID from
// the data source and verifies its result is fetched.
func TestAcc_DataSourceAction(t *testing.T) {
	// Actions are only tested on LXD. There is nothing cloud-specific
	// about actions, so running them on other clouds adds no value.
	if testingCloud != LXDCloudTesting {
		t.Skipf("skipping action test on %s cloud", testingCloud)
	}

	modelName := acctest.RandomWithPrefix("tf-test-action-ds")

	// Pick the charm and action depending on the cloud.
	charmName := "juju-qa-test"
	actionName := "fortune"
	trust := false
	// juju-qa-test's latest/stable does not support ubuntu@22.04, so let
	// juju resolve the base automatically.
	base := ""

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// The data source fetches the result of an action run
				// via the resource, keyed by its action_id.
				Config: testAccDataSourceAction(modelName, charmName, actionName, trust, base),
				Check: resource.ComposeTestCheckFunc(
					// The data source's model_uuid and action_id match
					// the action that was run.
					resource.TestCheckResourceAttrPair("data.juju_action.this", "model_uuid", "juju_action.this", "model_uuid"),
					resource.TestCheckResourceAttrPair("data.juju_action.this", "action_id", "juju_action.this", "action_id"),
					// The output is fetched and non-empty.
					resource.TestCheckResourceAttrSet("data.juju_action.this", "output"),
					// The data source's output matches the resource's.
					resource.TestCheckResourceAttrPair("data.juju_action.this", "output", "juju_action.this", "output"),
					resource.TestCheckResourceAttrSet("data.juju_action.this", "id"),
				),
			},
		},
	})
}

// testAccDataSourceAction builds a Terraform config that runs an action via
// the juju_action resource and reads its result back through the juju_action
// data source. The base parameter is optional; an empty value is omitted from
// the generated config.
func testAccDataSourceAction(modelName, charmName, actionName string, trust bool, base string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccDataSourceAction",
		`
resource "juju_model" "this" {
  name = "{{.ModelName}}"
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "test-app"

  charm {
    name = "{{.CharmName}}"
    {{ if .Base }}base = "{{.Base}}"{{ end }}
  }
  {{ if .Trust }}trust = true{{ end }}
}

resource "juju_action" "this" {
  model_uuid       = juju_model.this.uuid
  application_name = juju_application.this.name
  action_name      = "{{.ActionName}}"
  unit             = "test-app/leader"
}

data "juju_action" "this" {
  model_uuid = juju_model.this.uuid
  action_id  = juju_action.this.action_id
}
`, internaltesting.TemplateData{
			"ModelName":  modelName,
			"CharmName":  charmName,
			"ActionName": actionName,
			"Trust":      trust,
			"Base":       base,
		},
	)
}
