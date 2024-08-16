// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/names/v5"
)

func TestAcc_ResourceJaasAccessModel(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessSuccess := "writer"
	accessFail := "bogus"
	userOne := "foo@domain.com"
	userTwo := "bar@domain.com"
	var modelUUID string

	resourceName := "juju_jaas_access_model.test"

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set of users.
	// Test 2: Test updating the users to remove 1 user.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasModelAccess(userOne, accessSuccess, &modelUUID, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessModelTwoUsers(modelName, accessFail, userOne, userTwo),
				ExpectError: regexp.MustCompile(fmt.Sprintf("unknown relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessModelTwoUsers(modelName, accessSuccess, userOne, userTwo),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckModelUUIDNotEmpty(resourceName, &modelUUID),
					testAccCheckJaasModelAccess(userOne, accessSuccess, &modelUUID, true),
					testAccCheckJaasModelAccess(userTwo, accessSuccess, &modelUUID, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "foo@domain.com"),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", "bar@domain.com"),
					resource.TestCheckResourceAttr(resourceName, "users.#", "2"),
				),
			},
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessSuccess, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckJaasModelAccess(userOne, accessSuccess, &modelUUID, true),
					testAccCheckJaasModelAccess(userTwo, accessSuccess, &modelUUID, false),
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
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessAdmin := "administrator"
	userOne := "foo@domain.com"
	var modelUUID string

	resourceName := "juju_jaas_access_model.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasModelAccess(userOne, accessAdmin, &modelUUID, false),
			// TODO(Kian): The owner keeps access to the model after the destroy model command is
			// issued so that they can monitor the progress. Determine if there is a way to ensure
			// that relation is also eventually removed.
			// testAccCheckJaasModelAccess(expectedResourceOwner(), accessAdmin, &modelUUID, false),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessAdmin, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckModelUUIDNotEmpty(resourceName, &modelUUID),
					testAccCheckJaasModelAccess(userOne, accessAdmin, &modelUUID, true),
					testAccCheckJaasModelAccess(expectedResourceOwner(), accessAdmin, &modelUUID, true),
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
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessWriter := "writer"
	accessReader := "reader"
	userOne := "foo@domain.com"
	var modelUUID string

	resourceName := "juju_jaas_access_model.test"

	// Test 1: Test adding a valid user.
	// Test 2: Test updating model access string and see the resource will be replaced.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasModelAccess(userOne, accessWriter, &modelUUID, false),
		),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasAccessModelOneUser(modelName, accessWriter, userOne),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckModelUUIDNotEmpty(resourceName, &modelUUID),
					testAccCheckJaasModelAccess(userOne, accessWriter, &modelUUID, true),
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
	modelName := acctest.RandomWithPrefix("tf-jaas-access-model")
	accessSuccess := "writer"
	svcAccountOne := "foo-1"
	svcAccountTwo := "foo-2"
	user := "bob@domain.com"
	svcAccountOneWithDomain := svcAccountOne + "@serviceaccount"
	svcAccountTwoWithDomain := svcAccountTwo + "@serviceaccount"
	var modelUUID string

	resourceName := "juju_jaas_access_model.test"

	// Test 0: Test adding an invalid service account tag
	// Test 0: Test adding a valid service account.
	// Test 1: Test adding an additional service account and user.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasModelAccess(svcAccountOneWithDomain, accessSuccess, &modelUUID, false),
			testAccCheckJaasModelAccess(svcAccountTwoWithDomain, accessSuccess, &modelUUID, false),
			testAccCheckJaasModelAccess(user, accessSuccess, &modelUUID, false),
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
					testAccCheckModelUUIDNotEmpty(resourceName, &modelUUID),
					testAccCheckJaasModelAccess(svcAccountOneWithDomain, accessSuccess, &modelUUID, true),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "service_accounts.*", svcAccountOne),
					resource.TestCheckResourceAttr(resourceName, "service_accounts.#", "1"),
				),
			},
			{
				Config: testAccResourceJaasAccessModelSvcAccsAndUser(modelName, accessSuccess, user, svcAccountOne, svcAccountTwo),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckModelUUIDNotEmpty(resourceName, &modelUUID),
					testAccCheckJaasModelAccess(user, accessSuccess, &modelUUID, true),
					testAccCheckJaasModelAccess(svcAccountOneWithDomain, accessSuccess, &modelUUID, true),
					testAccCheckJaasModelAccess(svcAccountTwoWithDomain, accessSuccess, &modelUUID, true),
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
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
  name = %q
}

resource "juju_jaas_access_model" "test" {
  model_uuid   = juju_model.test-model.id
  access       = %q
  users        = [%q, %q]
}`, modelName, access, userOne, userTwo)
}

func testAccResourceJaasAccessModelOneUser(modelName, access, userOne string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
  name = %q
}

resource "juju_jaas_access_model" "test" {
  model_uuid   = juju_model.test-model.id
  access       = %q
  users        = [%q]
}`, modelName, access, userOne)
}

func testAccResourceJaasAccessModelOneSvcAccount(modelName, access, svcAccOne string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
  name = %q
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = %q
  service_accounts    = [%q]
}`, modelName, access, svcAccOne)
}

func testAccResourceJaasAccessModelSvcAccsAndUser(modelName, access, user, svcAccOne, svcAccTwo string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
  name = %q
}

resource "juju_jaas_access_model" "test" {
  model_uuid          = juju_model.test-model.id
  access              = %q
  users               = [%q]
  service_accounts    = [%q, %q]
}`, modelName, access, user, svcAccOne, svcAccTwo)
}

func testAccCheckModelUUIDNotEmpty(resourceName string, modelUUID *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// retrieve the resource by name from state
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", resourceName)
		}

		val, ok := rs.Primary.Attributes["model_uuid"]
		if !ok {
			return fmt.Errorf("Model UUID is not set")
		}
		if val == "" {
			return fmt.Errorf("Model UUID is empty")
		}
		if modelUUID == nil {
			return fmt.Errorf("cannot set model UUID, nil poiner")
		}
		*modelUUID = val
		return nil
	}
}

func testAccCheckJaasModelAccess(user, relation string, modelUUID *string, expectedAccess bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if modelUUID == nil {
			return fmt.Errorf("no model UUID set")
		}
		conn, err := TestClient.Models.GetConnection(nil)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		jc := api.NewClient(conn)
		req := params.CheckRelationRequest{
			Tuple: params.RelationshipTuple{
				Object:       names.NewUserTag(user).String(),
				Relation:     relation,
				TargetObject: names.NewModelTag(*modelUUID).String(),
			},
		}
		resp, err := jc.CheckRelation(&req)
		if err != nil {
			return err
		}
		if resp.Allowed != expectedAccess {
			var access string
			if expectedAccess {
				access = "access"
			} else {
				access = "no access"
			}
			return fmt.Errorf("expected %s for user %s as %s to model (%s), but access is %t", access, user, relation, *modelUUID, resp.Allowed)
		}
		return nil
	}
}
