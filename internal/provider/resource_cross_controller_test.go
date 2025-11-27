// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// These tests require a Juju controller and an existing offering Juju controler with a model, an
// application and an offer. Check `project-docs/CROSS_CONTROLLER_TESTS.md` for more details
// on how to set up the environment.

type offeringControllerData struct {
	ControllerName         string
	ControllerAddr         string
	ControllerUser         string
	ControllerPass         string
	ControllerCert         string
	ControllerClientID     string
	ControllerClientSecret string
}

func TestAcc_ResourceIntegration_CrossControllers_Basic(t *testing.T) {
	OnlyCrossController(t)
	consumerModel := acctest.RandomWithPrefix("tf-integration-consumer-cross-controller")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationCrossController(consumerModel, getOfferingControllerDataFromEnv(t)),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.consumer", "uuid", "juju_integration.a", "model_uuid"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_integration.a",
			},
		},
	})
}

func testAccResourceIntegrationCrossController(ConsumerModel string, offeringController offeringControllerData) string {
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
  
resource "juju_model" "consumer" {
	name = "{{ .ConsumerModel }}"
}

resource "juju_application" "consumer" {
	model_uuid = juju_model.consumer.uuid
	name  = "consumer"
	
	charm {
		name = "juju-qa-dummy-sink"
	}
}


resource "juju_integration" "a" {
	model_uuid = juju_model.consumer.uuid

	application {
		name = juju_application.consumer.name
		endpoint = "source"
	}
	
	application {
		offering_controller = "{{.ControllerName}}"
		{{if .ControllerClientID}}
		offer_url = "jimm-test@canonical.com/offering-model.dummy-source"
		{{else}}
		offer_url = "admin/offering-model.dummy-source" # offer url from offering controller
		{{end}}
	}
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

func getOfferingControllerDataFromEnv(t *testing.T) offeringControllerData {
	controllerName, ok := os.LookupEnv("OFFERING_CONTROLLER_NAME")
	if !ok {
		t.Fatalf("OFFERING_CONTROLLER_NAME environment variable not set")
	}

	controllerAddr, ok := os.LookupEnv("OFFERING_CONTROLLER_ADDRESSES")
	if !ok {
		t.Fatalf("OFFERING_CONTROLLER_ADDRESSES environment variable not set")
	}

	controllerCert, ok := os.LookupEnv("OFFERING_CONTROLLER_CA_CERT")
	if !ok {
		t.Fatalf("OFFERING_CONTROLLER_CA_CERT environment variable not set")
	}

	controllerUser, _ := os.LookupEnv("OFFERING_CONTROLLER_USERNAME")
	controllerPass, _ := os.LookupEnv("OFFERING_CONTROLLER_PASSWORD")
	if controllerUser != "" && controllerPass != "" {
		return offeringControllerData{
			ControllerName: controllerName,
			ControllerAddr: controllerAddr,
			ControllerUser: controllerUser,
			ControllerPass: controllerPass,
			ControllerCert: controllerCert,
		}
	}

	controllerClientID, ok := os.LookupEnv("OFFERING_CONTROLLER_CLIENT_ID")
	if !ok {
		t.Fatalf("OFFERING_CONTROLLER_CLIENT_ID environment variable not set")
	}

	controllerClientSecret, ok := os.LookupEnv("OFFERING_CONTROLLER_CLIENT_SECRET")
	if !ok {
		t.Fatalf("OFFERING_CONTROLLER_CLIENT_SECRET environment variable not set")
	}

	return offeringControllerData{
		ControllerName:         controllerName,
		ControllerAddr:         controllerAddr,
		ControllerCert:         controllerCert,
		ControllerClientID:     controllerClientID,
		ControllerClientSecret: controllerClientSecret,
	}
}
