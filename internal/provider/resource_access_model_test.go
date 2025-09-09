// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceAccessModel(t *testing.T) {
	SkipJAAS(t)
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	userName2 := acctest.RandomWithPrefix("tfuser")
	userPassword2 := acctest.RandomWithPrefix("tf-test-user")
	modelName1 := acctest.RandomWithPrefix("tf-access-model-one")
	modelName2 := acctest.RandomWithPrefix("tf-access-model-two")
	accessSuccess := "write"
	accessFail := "bogus"

	resourceName := "juju_access_model.test"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessModel(userName, userPassword, modelName1, accessFail),
				ExpectError: regexp.MustCompile("Invalid Attribute Value Match.*"),
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
	SkipJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := acctest.RandomWithPrefix("tf-access-model")
	access := "write"

	resourceName := "juju_access_model.test"
	resource.ParallelTest(t, resource.TestCase{
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

func TestAcc_ResourceAccessModel_ErrorWhenUsedWithJAAS(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessModelFixedUser(),
				ExpectError: regexp.MustCompile("This resource is not supported with JAAS"),
			},
		},
	})
}

func testAccResourceAccessModelFixedUser() string {
	return `
resource "juju_access_model" "test" {
  access = "write"
  model = "foo"
  users = ["bob"]
}`
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

func TestAcc_ResourceAccessModel_Schema_v0_To_v1(t *testing.T) {
	SkipJAAS(t)
	user1 := acctest.RandomWithPrefix("tfuser1")
	password1 := acctest.RandomWithPrefix("tf-test-user1")
	user2 := acctest.RandomWithPrefix("tfuser2")
	password2 := acctest.RandomWithPrefix("tf-test-user2")
	modelName := acctest.RandomWithPrefix("tf-access-model")

	resourceName := "juju_access_model.test"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: "0.21.1",
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, "write"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckResourceAttr(resourceName, "access", "write"),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", user1),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", user2),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, "write"),
			},
		},
	})
}

func testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, access string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceModel",
		`
resource "juju_user" "test-user1" {
  name = "{{.User1}}"
  password = "{{.Password1}}"
}

resource "juju_user" "test-user2" {
  name = "{{.User2}}"
  password = "{{.Password2}}"
}

resource "juju_model" "test-model" {
  name = "{{.ModelName}}"
}

resource "juju_access_model" "test" {
  access = "{{.Access}}"
  model = juju_model.test-model.name

  users = [juju_user.test-user1.name, juju_user.test-user2.name]
}`, internaltesting.TemplateData{
			"ModelName": modelName,
			"User1":     user1,
			"Password1": password1,
			"User2":     user2,
			"Password2": password2,
			"Access":    access,
		})
}
