// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// TestAcc_ResourceAction tests the juju_action resource. It deploys a single
// model+application and exercises the leader, specific-unit and args
// variants across multiple steps.
func TestAcc_ResourceAction(t *testing.T) {
	// Actions are only tested on LXD. There is nothing cloud-specific
	// about actions, so running them on other clouds adds no value.
	if testingCloud != LXDCloudTesting {
		t.Skipf("skipping action test on %s cloud", testingCloud)
	}

	charmName := "juju-qa-dummy-source"
	actionName := "echo"
	modelName := acctest.RandomWithPrefix("tf-test-action")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Run on the leader unit.
				Config: testAccResourceAction(modelName, charmName, actionName, "test-app/leader", `"value" = "ciao"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_action.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_action.this", "action_name", actionName),
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestMatchResourceAttr("juju_action.this", "output", regexp.MustCompile(`"echo":\{"value":"ciao"\}`)),
				),
			},
			{
				// Run on a specific unit (forces retrigger via RequiresReplace).
				Config: testAccResourceAction(modelName, charmName, actionName, "test-app/0", `"value" = "ciao"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_action.this", "unit", "test-app/0"),
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestMatchResourceAttr("juju_action.this", "output", regexp.MustCompile(`"echo":\{"value":"ciao"\}`)),
				),
			},
			{
				// Run with different arguments (forces retrigger via RequiresReplace).
				Config: testAccResourceAction(modelName, charmName, actionName, "test-app/0", `"value" = "world"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestMatchResourceAttr("juju_action.this", "output", regexp.MustCompile(`"echo":\{"value":"world"\}`)),
				),
			},
			{
				// Running an action that does not exist produces an error.
				Config:      testAccResourceAction(modelName, charmName, "nonexistent-action", "test-app/leader", ""),
				ExpectError: regexp.MustCompile(`is not defined on the charm`),
			},
		},
	})
}

// testAccResourceAction builds a Terraform config for the juju_action resource.
// The args parameter is optional; empty values are omitted from the generated
// config. The unit parameter is always required.
func testAccResourceAction(modelName, charmName, actionName, unit, args string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceAction",
		`
resource "juju_model" "this" {
  name = "{{.ModelName}}"
}

resource "juju_application" "this" {
  model_uuid = juju_model.this.uuid
  name       = "test-app"

  charm {
    name = "{{.CharmName}}"
  }
}

resource "juju_action" "this" {
  model_uuid       = juju_model.this.uuid
  application_name = juju_application.this.name
  action_name      = "{{.ActionName}}"
  unit             = "{{.Unit}}"
  {{ if .Args }}args = {
    {{.Args}}
  }{{ end }}
}
`, internaltesting.TemplateData{
			"ModelName":  modelName,
			"CharmName":  charmName,
			"ActionName": actionName,
			"Unit":       unit,
			"Args":       args,
		},
	)
}
