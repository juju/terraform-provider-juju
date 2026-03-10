// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// The test steps are defined below to live close to the logic
// and used in the tests where we bootstrap a controller to
// avoid bootstrapping twice in multiple tests.

func testJAASControllerResourceSteps(t *testing.T, resourceName, controllerName string, bootstrapConfig map[string]string) []resource.TestStep {
	return []resource.TestStep{{
		SkipFunc: func() (bool, error) {
			// Skip if not JAAS
			if _, ok := os.LookupEnv("IS_JAAS"); !ok {
				return true, nil
			}
			return false, nil
		},
		Config: testAccResourceControllerAndJAASRegistration(
			controllerName,
			bootstrapConfig,
			nil,
			nil,
		),
		Check: resource.ComposeTestCheckFunc(
			resource.TestCheckResourceAttr("juju_jaas_controller.jaas", "name", controllerName),
			resource.TestCheckResourceAttrPair("juju_jaas_controller.jaas", "uuid", resourceName, "controller_uuid"),
			resource.TestCheckResourceAttrSet("juju_jaas_controller.jaas", "id"),
			resource.TestCheckResourceAttrSet("juju_jaas_controller.jaas", "status"),
			testAccCheckJaasControllerRegistered(t, controllerName, true),
		),
	},
		{
			SkipFunc: func() (bool, error) {
				// Skip if not JAAS
				if _, ok := os.LookupEnv("IS_JAAS"); !ok {
					return true, nil
				}
				return false, nil
			},
			Config: testAccResourceControllerAndJAASRegistration(
				controllerName,
				bootstrapConfig,
				nil,
				nil,
			),
			ResourceName:      "juju_jaas_controller.jaas",
			ImportState:       true,
			ImportStateVerify: false, // If importing some values cannot be obtained like the username/password.
		},
	}
}

func testAccResourceControllerAndJAASRegistration(controllerName string, bootstrapConfig, controllerConfig, modelConfig map[string]string) string {
	base := testAccResourceControllerWithJujuBinary(controllerName, bootstrapConfig, controllerConfig, modelConfig)
	return base + `
resource "juju_jaas_controller" "jaas" {
  name = juju_controller.controller.name
  uuid = juju_controller.controller.controller_uuid

  api_addresses  = juju_controller.controller.api_addresses
  ca_certificate = juju_controller.controller.ca_cert

  username = juju_controller.controller.username
  password = juju_controller.controller.password

  tls_hostname = "juju-apiserver"
}
`
}
