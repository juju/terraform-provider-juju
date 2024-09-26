// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"testing"

	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/names/v5"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// This file has bare minimum tests for controller access
// verifying that users, service accounts and groups
// can access a controller. More extensive tests for
// generic jaas access are available in
// resource_access_jaas_model_test.go

func TestAcc_ResourceJaasAccessController(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// Resource names
	controllerAccessResourceName := "juju_jaas_access_controller.test"
	groupResourcename := "juju_jaas_group.test"
	accessSuccess := "administrator"
	accessFail := "bogus"
	user := "foo@domain.com"
	group := acctest.RandomWithPrefix("myGroup")
	svcAcc := "test"
	svcAccWithDomain := svcAcc + "@serviceaccount"

	// Objects for checking access
	groupRelationF := func(s string) string { return jimmnames.NewGroupTag(s).String() + "#member" }
	groupCheck := newCheckAttribute(groupResourcename, "uuid", groupRelationF)
	userTag := names.NewUserTag(user).String()
	svcAccTag := names.NewUserTag(svcAccWithDomain).String()
	controllerTag := names.NewControllerTag("jimm").String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set user, group and service account.
	// Test 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, &controllerTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &controllerTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &controllerTag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessController(accessFail, user, group, svcAcc),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessController(accessSuccess, user, group, svcAcc),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(groupCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, &controllerTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &controllerTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &controllerTag, true),
					resource.TestCheckResourceAttr(controllerAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(controllerAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(controllerAccessResourceName, "users.#", "1"),
					// Wrap this check so that the pointer has deferred evaluation.
					func(s *terraform.State) error {
						return resource.TestCheckTypeSetElemAttr(controllerAccessResourceName, "groups.*", *groupCheck.resourceID)(s)
					},
					resource.TestCheckResourceAttr(controllerAccessResourceName, "groups.#", "1"),
					resource.TestCheckTypeSetElemAttr(controllerAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(controllerAccessResourceName, "service_accounts.#", "1"),
				),
				// The plan will not be empty because JAAS will have some default admin users.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAcc_ResourceJaasAccessControllerImportState(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	access := "administrator"

	resourceName := "juju_jaas_access_controller.test"

	// Test 0: Test importing works.
	// Note that there is, by default, 1 superuser created for JAAS in the setup action (jimm-test@canonical).
	// Additionally, a service account is created an also made a JAAS administrator.
	// We verify that this user/svcAcc are present in the imported plan.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccResourceJaasAccessControllerAdmins(access),
				PlanOnly:          true,
				ImportStateVerify: false,
				ImportStateCheck: func(is []*terraform.InstanceState) error {
					if len(is) != 1 {
						return errors.New("expected 1 instance in import state")
					}
					state := is[0]
					checker := func(key, expected string) error {
						if value, ok := state.Attributes[key]; !ok {
							return fmt.Errorf("did not find attribute %s", key)
						} else if value != expected {
							return fmt.Errorf("value for attribute %s did not match, got %s expected %s", key, value, expected)
						}
						return nil
					}
					errs := make([]error, 1)
					errs = append(errs, checker("users.0", "jimm-test@canonical.com"))
					errs = append(errs, checker("users.#", "1"))
					errs = append(errs, checker("service_accounts.0", strings.TrimSuffix(expectedResourceOwner(), "@serviceaccount")))
					errs = append(errs, checker("service_accounts.#", "1"))
					return errors.Join(errs...)
				},
				ImportState:   true,
				ImportStateId: fmt.Sprintf("jimm:%s", access),
				ResourceName:  resourceName,
			},
		},
	})
}

func testAccResourceJaasAccessController(access, user, group, svcAcc string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessController",
		`
resource "juju_jaas_group" "test" {
  name = "{{ .Group }}"
}

resource "juju_jaas_access_controller" "test" {
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  groups              = [juju_jaas_group.test.uuid]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"Access": access,
			"User":   user,
			"Group":  group,
			"SvcAcc": svcAcc,
		})
}

func testAccResourceJaasAccessControllerAdmins(access string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessControllerEmpty",
		`
resource "juju_jaas_access_controller" "test" {
  access              = "{{.Access}}"
  users               = ["foo@external"]
}
`, internaltesting.TemplateData{
			"Access": access,
		})
}
