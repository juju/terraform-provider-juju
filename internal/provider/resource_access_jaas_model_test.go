// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/juju/names/v5"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceJaasAccessModel(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	// Resource names
	resourceName := "juju_jaas_access_model.test"
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessSuccess := "writer"
	accessFail := "bogus"
	userOne := "foo@domain.com"
	userTwo := "bar@domain.com"

	// Objects for checking access
	newModelTagF := func(s string) string { return names.NewModelTag(s).String() }
	modelCheck := newCheckAttribute(resourceName, "model_uuid", newModelTagF)
	userOneTag := names.NewUserTag(userOne).String()
	userTwoTag := names.NewUserTag(userTwo).String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set of users.
	// Test 2: Test importing works
	// Test 3: Test updating the users to remove 1 user.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userOneTag, modelCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessModelTwoUsers(modelName, accessFail, userOne, userTwo),
				ExpectError: regexp.MustCompile(fmt.Sprintf("unknown relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessModelTwoUsers(modelName, accessSuccess, userOne, userTwo),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(modelCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userOneTag, modelCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &userTwoTag, modelCheck.tag, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "bar@domain.com"),
					resource.TestCheckResourceAttr(resourceName, "users.#", "2"),
				),
			},
			{
				Destroy:           false,
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessSuccess, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckJaasResourceAccess(accessSuccess, &userOneTag, modelCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &userTwoTag, modelCheck.tag, false),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
					resource.TestCheckResourceAttr(resourceName, "users.#", "1"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

// TestAcc_ResourceJaasAccessModelAdmin verifies behaviour when setting admin access.
// When a model is created, it is expected that the model owner is also a model admin.
// Test that the refresh plan is not empty if the model owner is not included and verify
// that the model owner has access to the model.
func TestAcc_ResourceJaasAccessModelAdmin(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	expectedResourceOwner()

	// Resource names
	resourceName := "juju_jaas_access_model.test"
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessAdmin := "administrator"
	userOne := "foo@domain.com"

	// Objects for checking access
	resourceOwnerTag := names.NewUserTag(expectedResourceOwner()).String()
	newModelTagF := func(s string) string { return names.NewModelTag(s).String() }
	modelCheck := newCheckAttribute(resourceName, "model_uuid", newModelTagF)
	userOneTag := names.NewUserTag(userOne).String()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessAdmin, &userOneTag, modelCheck.tag, false),
			// TODO(Kian): The owner keeps access to the model after the destroy model command is
			// issued so that they can monitor the progress. Determine if there is a way to ensure
			// that relation is also eventually removed.
			// testAccCheckJaasModelAccess(expectedResourceOwner(), accessAdmin, &modelUUID, false),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessAdmin, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(modelCheck),
					testAccCheckJaasResourceAccess(accessAdmin, &userOneTag, modelCheck.tag, true),
					testAccCheckJaasResourceAccess(accessAdmin, &resourceOwnerTag, modelCheck.tag, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessAdmin),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
					resource.TestCheckResourceAttr(resourceName, "users.#", "1"),
				),
				ExpectError: regexp.MustCompile(`.*the refresh plan was not empty\.`),
			},
		},
	})
}

func TestAcc_ResourceJaasAccessModelChangingAccessReplacesResource(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	// Resource names
	resourceName := "juju_jaas_access_model.test"
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessWriter := "writer"
	accessReader := "reader"
	userOne := "foo@domain.com"

	// Objects for checking access
	newModelTagF := func(s string) string { return names.NewModelTag(s).String() }
	modelCheck := newCheckAttribute(resourceName, "model_uuid", newModelTagF)
	userOneTag := names.NewUserTag(userOne).String()

	// Test 1: Test adding a valid user.
	// Test 2: Test updating model access string and see the resource will be replaced.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessWriter, &userOneTag, modelCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessWriter, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(modelCheck),
					testAccCheckJaasResourceAccess(accessWriter, &userOneTag, modelCheck.tag, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessWriter),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
					resource.TestCheckResourceAttr(resourceName, "users.#", "1"),
				),
			},
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessReader, userOne),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						ExpectRecreatedResource(resourceName),
					},
				},
			},
		},
	})
}

func TestAcc_ResourceJaasAccessModelServiceAccountAndUsers(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	// Resource names
	resourceName := "juju_jaas_access_model.test"
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessSuccess := "writer"
	svcAccountOne := "foo-1"
	svcAccountTwo := "foo-2"
	user := "bob@domain.com"

	// Objects for checking access
	newModelTagF := func(s string) string { return names.NewModelTag(s).String() }
	modelCheck := newCheckAttribute(resourceName, "model_uuid", newModelTagF)
	userTag := names.NewUserTag(user).String()
	svcAccOneTag := names.NewUserTag(svcAccountOne + "@serviceaccount").String()
	svcAccTwoTag := names.NewUserTag(svcAccountTwo + "@serviceaccount").String()

	// Test 0: Test adding an invalid service account tag
	// Test 0: Test adding a valid service account.
	// Test 1: Test adding an additional service account and user.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccOneTag, modelCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTwoTag, modelCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, modelCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasAccessModelOneSvcAccount(modelName, accessSuccess, "##invalid-svc-acc-id##"),
				// The regex below may break because of changes in formatting/line breaks in the TF output.
				ExpectError: regexp.MustCompile(".*ID must be a valid Juju username.*"),
			},
			{
				Config: testAccResourceJaasAccessModelOneSvcAccount(modelName, accessSuccess, svcAccountOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(modelCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccOneTag, modelCheck.tag, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "service_accounts.*", svcAccountOne),
					resource.TestCheckResourceAttr(resourceName, "service_accounts.#", "1"),
				),
			},
			{
				Config: testAccResourceJaasAccessModelSvcAccsAndUser(modelName, accessSuccess, user, svcAccountOne, svcAccountTwo),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(modelCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, modelCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccOneTag, modelCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTwoTag, modelCheck.tag, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", user),
					resource.TestCheckTypeSetElemAttr(resourceName, "service_accounts.*", svcAccountOne),
					resource.TestCheckTypeSetElemAttr(resourceName, "service_accounts.*", svcAccountTwo),
					resource.TestCheckResourceAttr(resourceName, "users.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "service_accounts.#", "2"),
				),
			},
		},
	})
}

// TODO(Kian): Add the test below after a stable release of the provider that includes jaas resources.

// func TestAcc_ResourceJaasAccessModel_UpgradeProvider(t *testing.T) {
// 	OnlyTestAgainstJAAS(t)
// 	if testingCloud != LXDCloudTesting {
// 		t.Skip(t.Name() + " only runs with LXD")
// 	}

// 	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
// 	accessSuccess := "writer"

// 	resourceName := "juju_access_model.test"
// 	resource.ParallelTest(t, resource.TestCase{
// 		PreCheck: func() { testAccPreCheck(t) },

// 		Steps: []resource.TestStep{
// 			{
// 				ExternalProviders: map[string]resource.ExternalProvider{
// 					"juju": {
// 						VersionConstraint: TestProviderStableVersion,
// 						Source:            "juju/juju",
// 					},
// 				},
// 				Config: testAccResourceJaasAccessModel(modelName, accessSuccess),
// 				Check: resource.ComposeTestCheckFunc(
// 					resource.TestMatchResourceAttr(resourceName, "model_uuid", regexp.MustCompile(".+")),
// 					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
// 					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
// 					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "bar@domain.com"),
// 				),
// 			},
// 			{
// 				ProtoV6ProviderFactories: frameworkProviderFactories,
// 				Config:                   testAccResourceJaasAccessModel(modelName, accessSuccess),
// 				PlanOnly:                 true,
// 			},
// 		},
// 	})
// }

func testAccResourceJaasAccessModelTwoUsers(modelName, access, userOne, userTwo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessModelTwoUsers",
		`
resource "juju_model" "test-model" {
  name = "{{.ModelName}}"
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = "{{.Access}}"
  users               = ["{{.UserOne}}", "{{.UserTwo}}"]
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Access":    access,
			"UserOne":   userOne,
			"UserTwo":   userTwo,
		})
}

func testAccResourceJaasAccessModelOneUser(modelName, access, user string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessModelOneUser",
		`
resource "juju_model" "test-model" {
  name = "{{.ModelName}}"
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Access":    access,
			"User":      user,
		})
}

func testAccResourceJaasAccessModelOneSvcAccount(modelName, access, svcAcc string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessModelOneSvcAccount",
		`
resource "juju_model" "test-model" {
  name = "{{.ModelName}}"
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = "{{.Access}}"
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Access":    access,
			"SvcAcc":    svcAcc,
		})
}

func testAccResourceJaasAccessModelSvcAccsAndUser(modelName, access, user, svcAccOne, svcAccTwo string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessModelSvcAccsAndUser",
		`
resource "juju_model" "test-model" {
  name = "{{.ModelName}}"
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  service_accounts    = ["{{.SvcAccOne}}", "{{.SvcAccTwo}}"]
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Access":    access,
			"User":      user,
			"SvcAccOne": svcAccOne,
			"SvcAccTwo": svcAccTwo,
		})
}
