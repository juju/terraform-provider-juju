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

// This file has bare minimum tests for group access
// verifying that users, service accounts and groups
// can access a group. More extensive tests for
// generic jaas access are available in
// resource_access_jaas_model_test.go

func TestAcc_ResourceJaasAccessGroup(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// Resource names, note that group two has access to group one.
	groupAccessResourceName := "juju_jaas_access_group.test"

	groupOneResourcename := "juju_jaas_group.test"
	groupTwoResourceName := "juju_jaas_group.groupWithAccess"
	accessSuccess := "member"
	accessFail := "bogus"
	user := "foo@domain.com"
	groupOneName := acctest.RandomWithPrefix("group1")
	groupTwoName := acctest.RandomWithPrefix("group2")
	svcAcc := "test"
	svcAccWithDomain := svcAcc + "@serviceaccount"

	// Objects for checking access
	groupRelationF := func(s string) string { return jimmnames.NewGroupTag(s).String() }
	groupOneCheck := newCheckAttribute(groupOneResourcename, "uuid", groupRelationF)
	groupWithMemberRelationF := func(s string) string { return jimmnames.NewGroupTag(s).String() + "#member" }
	groupTwoCheck := newCheckAttribute(groupTwoResourceName, "uuid", groupWithMemberRelationF)
	userTag := names.NewUserTag(user).String()
	svcAccTag := names.NewUserTag(svcAccWithDomain).String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set user, group and service account.
	// Test 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, groupOneCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, groupTwoCheck.tag, groupOneCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, groupOneCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessGroup(groupOneName, accessFail, user, groupTwoName, svcAcc),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessGroup(groupOneName, accessSuccess, user, groupTwoName, svcAcc),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(groupOneCheck),
					testAccCheckAttributeNotEmpty(groupTwoCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, groupOneCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, groupTwoCheck.tag, groupOneCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, groupOneCheck.tag, true),
					resource.TestCheckResourceAttr(groupAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(groupAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(groupAccessResourceName, "users.#", "1"),
					// Wrap this check so that the pointer has deferred evaluation.
					func(s *terraform.State) error {
						return resource.TestCheckTypeSetElemAttr(groupAccessResourceName, "groups.*", *groupTwoCheck.resourceID)(s)
					},
					resource.TestCheckResourceAttr(groupAccessResourceName, "groups.#", "1"),
					resource.TestCheckTypeSetElemAttr(groupAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(groupAccessResourceName, "service_accounts.#", "1"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      groupAccessResourceName,
			},
		},
	})
}

func testAccResourceJaasAccessGroup(groupName, access, user, groupWithAccess, svcAcc string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessGroup",
		`
resource "juju_jaas_group" "test" {
  name = "{{ .Group }}"
}

resource "juju_jaas_group" "groupWithAccess" {
  name = "{{ .GroupWithAccess }}"
}

resource "juju_jaas_access_group" "test" {
  group_id            = juju_jaas_group.test.uuid
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  groups              = [juju_jaas_group.groupWithAccess.uuid]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"Group":           groupName,
			"Access":          access,
			"User":            user,
			"GroupWithAccess": groupWithAccess,
			"SvcAcc":          svcAcc,
		})
}
