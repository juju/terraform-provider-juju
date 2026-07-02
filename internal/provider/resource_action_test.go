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
// model+application and exercises the optional args and unit variants across
// multiple steps.
func TestAcc_ResourceAction(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-action")

	// Pick the charm and action depending on the cloud.
	charmName := "juju-qa-test"
	actionName := "fortune"
	trust := false
	// juju-qa-test's latest/stable does not support ubuntu@22.04, so let
	// juju resolve the base automatically.
	base := ""
	if testingCloud == MicroK8sTesting {
		charmName = "traefik-k8s"
		actionName = "show-proxied-endpoints"
		trust = true
		// traefik-k8s's latest/stable does not support ubuntu@22.04, so
		// let juju resolve the base automatically.
		base = ""
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Run on the leader unit (default).
				Config: testAccResourceAction(modelName, charmName, actionName, trust, base, "", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_action.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_action.this", "action_name", actionName),
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestCheckResourceAttrSet("juju_action.this", "output.%"),
				),
			},
			{
				// Run on a specific unit.
				Config: testAccResourceAction(modelName, charmName, actionName, trust, base, "test-app/0", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_action.this", "unit", "test-app/0"),
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestCheckResourceAttrSet("juju_action.this", "output.%"),
				),
			},
			{
				// Run with arguments (only on LXD, where the charm
				// exposes an action with args).
				Config: testAccResourceAction(modelName, "juju-qa-dummy-source", "echo", false, "ubuntu@22.04", "", `"message" = "hello"`),
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("juju_action.this", "action_id"),
					resource.TestCheckResourceAttrSet("juju_action.this", "output.%"),
				),
			},
			{
				// Running an action that does not exist produces an error.
				Config: testAccResourceAction(modelName, charmName, "nonexistent-action", trust, base, "", ""),
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				ExpectError: regexp.MustCompile(`Unable to enqueue action`),
			},
		},
	})
}

// testAccResourceAction builds a Terraform config for the juju_action resource.
// The base, unit and args parameters are optional; empty values are omitted
// from the generated config.
func testAccResourceAction(modelName, charmName, actionName string, trust bool, base, unit, args string) string {
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
    {{ if .Base }}base = "{{.Base}}"{{ end }}
  }
  {{ if .Trust }}trust = true{{ end }}
}

resource "juju_action" "this" {
  model_uuid       = juju_model.this.uuid
  application_name = juju_application.this.name
  action_name      = "{{.ActionName}}"
  {{ if .Unit }}unit = "{{.Unit}}"{{ end }}
  {{ if .Args }}args = {
    {{.Args}}
  }{{ end }}
}
`, internaltesting.TemplateData{
			"ModelName":  modelName,
			"CharmName":  charmName,
			"ActionName": actionName,
			"Trust":      trust,
			"Base":       base,
			"Unit":       unit,
			"Args":       args,
		},
	)
}
