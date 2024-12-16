// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/names/v5"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// This file has bare minimum tests for service account access
// verifying that users, service accounts and groups
// can access a service account. More extensive tests for
// generic jaas access are available in
// resource_access_jaas_model_test.go

func TestAcc_ResourceJaasAccessServiceAccount(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// Resource names
	svcAccAccessResourceName := "juju_jaas_access_service_account.test"
	groupResourcename := "juju_jaas_group.test"
	roleResourcename := "juju_jaas_role.test"
	accessSuccess := "administrator"
	accessFail := "bogus"
	user := "foo@domain.com"
	group := acctest.RandomWithPrefix("myGroup")
	role := acctest.RandomWithPrefix("role1")
	svcAcc := "test"
	svcAccWithDomain := svcAcc + "@serviceaccount"

	// Objects for checking access
	groupRelationF := func(s string) string { return jimmnames.NewGroupTag(s).String() + "#member" }
	groupCheck := newCheckAttribute(groupResourcename, "uuid", groupRelationF)
	roleRelationF := func(s string) string { return jimmnames.NewRoleTag(s).String() + "#assignee" }
	roleCheck := newCheckAttribute(roleResourcename, "uuid", roleRelationF)
	userTag := names.NewUserTag(user).String()
	svcAccTag := names.NewUserTag(svcAccWithDomain).String()
	targetSvcAccTag := jimmnames.NewServiceAccountTag("foo@serviceaccount").String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set user, group and service account.
	// Test 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, &targetSvcAccTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &targetSvcAccTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, roleCheck.tag, &targetSvcAccTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &targetSvcAccTag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessServiceAccount(accessFail, user, group, svcAcc, role),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessServiceAccount(accessSuccess, user, group, svcAcc, role),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(groupCheck),
					testAccCheckAttributeNotEmpty(roleCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, &targetSvcAccTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &targetSvcAccTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &targetSvcAccTag, true),
					resource.TestCheckResourceAttr(svcAccAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(svcAccAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(svcAccAccessResourceName, "users.#", "1"),
					// Wrap this check so that the pointer has deferred evaluation.
					func(s *terraform.State) error {
						return resource.TestCheckTypeSetElemAttr(svcAccAccessResourceName, "groups.*", *groupCheck.resourceID)(s)
					},
					resource.TestCheckResourceAttr(svcAccAccessResourceName, "groups.#", "1"),
					resource.TestCheckTypeSetElemAttr(svcAccAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(svcAccAccessResourceName, "service_accounts.#", "1"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      svcAccAccessResourceName,
			},
		},
	})
}

func testAccResourceJaasAccessServiceAccount(access, user, group, svcAcc, role string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessServiceAccount",
		`
resource "juju_jaas_role" "test" {
  name = "{{ .Role }}"
}

resource "juju_jaas_group" "test" {
  name = "{{ .Group }}"
}

resource "juju_jaas_access_service_account" "test" {
  service_account_id  = "foo"
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  groups              = [juju_jaas_group.test.uuid]
  roles              = [juju_jaas_role.test.uuid]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"Access": access,
			"User":   user,
			"Group":  group,
			"SvcAcc": svcAcc,
			"Role":   role,
		})
}
