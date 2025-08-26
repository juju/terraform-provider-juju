// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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
				Config:      testAccResourceAccessModel(userName, userPassword, modelName1, accessFail, true),
				ExpectError: regexp.MustCompile("Invalid Attribute Value Match.*"),
			},
			{
				Config: testAccResourceAccessModel(userName, userPassword, modelName1, accessSuccess, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceName, "model_uuid", "juju_model."+modelName1, "uuid"),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					id := rs.Primary.Attributes["model_uuid"]
					if id == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:%s:%s", id, accessSuccess, userName), nil
				},
				ResourceName: resourceName,
			},
			{
				Config: testAccResourceAccessModel(userName2, userPassword2,
					modelName2, accessSuccess, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckResourceAttrPair(resourceName, "model_uuid", "juju_model."+modelName2, "uuid"),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName2),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceName]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					id := rs.Primary.Attributes["model_uuid"]
					if id == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:%s:%s", id, accessSuccess, userName2), nil
				},
				ResourceName: resourceName,
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
				Config: testAccResourceAccessModel(userName, userPassword, modelName, access, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceAccessModel(userName, userPassword, modelName, access, true),
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
resource "juju_model" "test-model" {
	name = "test-model"
}
resource "juju_access_model" "test" {
  access = "write"
  model_uuid = juju_model.test-model.uuid
  users = ["bob"]
}`
}

func testAccResourceAccessModel(userName, userPassword, modelName, access string, useModelUUID bool) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceAccessModel",
		`resource "juju_user" "test-user" {
  name = "{{.UserName}}"
  password = "{{.UserPassword}}"
}

resource "juju_model" "{{.ModelName}}" {
  name = "{{.ModelName}}"
}

resource "juju_access_model" "test" {
  access = "{{.Access}}"
  {{- if eq .UseModelUUID false }}
  model = juju_model.{{.ModelName}}.name
  {{- else }}
  model_uuid = juju_model.{{.ModelName}}.uuid
  {{- end }}
  users = [juju_user.test-user.name]
}`, internaltesting.TemplateData{
			"UserName":     userName,
			"UserPassword": userPassword,
			"ModelName":    modelName,
			"Access":       access,
			"UseModelUUID": useModelUUID,
		})
}

func TestAcc_ResourceAccessModel_UpgradeV0ToV2(t *testing.T) {
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
				Config: testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, "write", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckResourceAttr(resourceName, "access", "write"),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", user1),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", user2),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, "write", true),
			},
		},
	})
}

func testAccResourceAccessModelTwoUsers(user1, password1, user2, password2, modelName, access string, useModelUUID bool) string {
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
  {{- if eq .UseModelUUID false }}
  model = juju_model.test-model.name
  {{- else }}
  model_uuid = juju_model.test-model.uuid
  {{- end }}

  users = [juju_user.test-user1.name, juju_user.test-user2.name]
}`, internaltesting.TemplateData{
			"ModelName":    modelName,
			"User1":        user1,
			"Password1":    password1,
			"User2":        user2,
			"Password2":    password2,
			"Access":       access,
			"UseModelUUID": useModelUUID,
		})
}

func TestAcc_ResourceAccessModel_UpgradeV1ToV2(t *testing.T) {
	SkipJAAS(t)

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
						VersionConstraint: TestProviderPreV1Version,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceAccessModel(userName, userPassword, modelName, access, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceAccessModel(userName, userPassword, modelName, access, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceName, "model_uuid", "juju_model."+modelName, "uuid"),
				),
			},
		},
	})
}
