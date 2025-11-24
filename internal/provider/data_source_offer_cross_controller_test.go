// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// These tests require a Juju controller and an existing offering Juju controller with a model, an
// application and an offer. Check `project-docs/CROSS_CONTROLLER_TESTS.md` for more details
// on how to set up the environment.

func TestAcc_DataOffer_CrossControllers_Basic(t *testing.T) {
	OnlyCrossController(t)
	consumerModel := acctest.RandomWithPrefix("tf-integration-consumer-cross-controller")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataOfferCrossController(consumerModel, getOfferingControllerDataFromEnv(t)),
			},
		},
	})
}

func testAccDataOfferCrossController(ConsumerModel string, offeringController offeringControllerData) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceIntegrationCrossController",
		`
provider "juju" {
  offering_controllers = {
    {{.ControllerName}} = {
		controller_addresses = "{{.ControllerAddr}}"
		{{if .ControllerClientID}}
		client_id = "{{.ControllerClientID}}"
		client_secret = "{{.ControllerClientSecret}}"
		{{else}}
		username = "{{.ControllerUser}}"
		password = "{{.ControllerPass}}"
		{{end}}
		ca_certificate = <<EOF
{{.ControllerCert}}
EOF
	}
  }
}
  
data "juju_offer" "this" {
	offering_controller = "{{.ControllerName}}"
	{{if .ControllerClientID}}
	url = "jimm-test@canonical.com/offering-model.dummy-source"
	{{else}}
	url = "admin/offering-model.dummy-source" # offer url from offering controller
	{{end}}
} 
`, internaltesting.TemplateData{
			"ConsumerModel":          ConsumerModel,
			"ControllerName":         offeringController.ControllerName,
			"ControllerAddr":         offeringController.ControllerAddr,
			"ControllerUser":         offeringController.ControllerUser,
			"ControllerPass":         offeringController.ControllerPass,
			"ControllerCert":         offeringController.ControllerCert,
			"ControllerClientID":     offeringController.ControllerClientID,
			"ControllerClientSecret": offeringController.ControllerClientSecret,
		})
}
