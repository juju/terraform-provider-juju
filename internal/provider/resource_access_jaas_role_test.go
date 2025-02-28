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
	"github.com/juju/names/v5"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// This file has bare minimum tests for role access
// verifying that users, service accounts and roles
// can access a role. More extensive tests for
// generic jaas access are available in
// resource_access_jaas_model_test.go

func TestAcc_ResourceJaasAccessRole(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// Resource names, note that role two has access to role one.
	RoleAccessResourceName := "juju_jaas_access_role.test"

	roleOneResourceName := "juju_jaas_role.test"
	accessSuccess := "assignee"
	accessFail := "bogus"
	user := "foo@domain.com"
	roleOneName := acctest.RandomWithPrefix("role1")
	svcAcc := "test"
	svcAccWithDomain := svcAcc + "@serviceaccount"

	// Objects for checking access
	RoleRelationF := func(s string) string { return jimmnames.NewRoleTag(s).String() }
	roleOneCheck := newCheckAttribute(roleOneResourceName, "uuid", RoleRelationF)
	UserTag := names.NewUserTag(user).String()
	svcAccTag := names.NewUserTag(svcAccWithDomain).String()

	// Step 0: Test an invalid access string.
	// Step 1: Test adding a valid set user, role and service account.
	// Step 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &UserTag, roleOneCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, roleOneCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessRole(roleOneName, accessFail, user, svcAcc),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessRole(roleOneName, accessSuccess, user, svcAcc),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(roleOneCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &UserTag, roleOneCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, roleOneCheck.tag, true),
					resource.TestCheckResourceAttr(RoleAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(RoleAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(RoleAccessResourceName, "users.#", "1"),
					resource.TestCheckTypeSetElemAttr(RoleAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(RoleAccessResourceName, "service_accounts.#", "1"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      RoleAccessResourceName,
			},
		},
	})
}

func testAccResourceJaasAccessRole(roleName, access, user, svcAcc string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessRole",
		`
resource "juju_jaas_role" "test" {
  name = "{{ .Role }}"
}

resource "juju_jaas_access_role" "test" {
  role_id            = juju_jaas_role.test.uuid
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"Role":   roleName,
			"Access": access,
			"User":   user,
			"SvcAcc": svcAcc,
		})
}
