// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceAccessModel(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	userName2 := acctest.RandomWithPrefix("tfuser")
	userPassword2 := acctest.RandomWithPrefix("tf-test-user")
	modelName1 := "testing1"
	modelName2 := "testing2"
	accessSuccess := "write"
	accessFail := "bogus"

	resourceName := "juju_access_model.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessModel(userName, userPassword, modelName1, accessFail),
				ExpectError: regexp.MustCompile("Error running pre-apply refresh.*"),
			},
			{
				Config: testAccResourceAccessModel(userName, userPassword, modelName1, accessSuccess),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName1),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName1, accessSuccess, userName),
				ResourceName:      resourceName,
			},
			{
				Config: testAccResourceAccessModel(userName2, userPassword2,
					modelName2, accessSuccess),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckResourceAttr(resourceName, "model", modelName2),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName2),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName2, accessSuccess, userName2),
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceAccessModel_UpgradeProvider(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := "testing"
	access := "write"

	resourceName := "juju_access_model.test"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceAccessModel(userName, userPassword, modelName, access),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceAccessModel(userName, userPassword, modelName, access),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceAccessModel(userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
resource "juju_user" "test-user" {
  name = %q
  password = %q
}

resource "juju_model" "test-model" {
  name = %q
}

resource "juju_access_model" "test" {
  access = %q
  model = juju_model.test-model.name
  users = [juju_user.test-user.name]
}`, userName, userPassword, modelName, access)
}
