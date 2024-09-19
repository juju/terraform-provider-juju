// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"errors"
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

func TestAcc_ResourceJaasAccessCloud(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	// Resource names
	cloudAccessResourceName := "juju_jaas_access_cloud.test"
	groupResourcename := "juju_jaas_group.test"
	cloudName := "localhost"
	accessSuccess := "can_addmodel"
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
	cloudTag := names.NewCloudTag(cloudName).String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set user, group and service account.
	// Test 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, &cloudTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &cloudTag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &cloudTag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessCloud(cloudName, accessFail, user, group, svcAcc),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessCloud(cloudName, accessSuccess, user, group, svcAcc),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(groupCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, &cloudTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, &cloudTag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, &cloudTag, true),
					resource.TestCheckResourceAttr(cloudAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(cloudAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(cloudAccessResourceName, "users.#", "1"),
					// Wrap this check so that the pointer has deferred evaluation.
					func(s *terraform.State) error {
						return resource.TestCheckTypeSetElemAttr(cloudAccessResourceName, "groups.*", *groupCheck.resourceID)(s)
					},
					resource.TestCheckResourceAttr(cloudAccessResourceName, "groups.#", "1"),
					resource.TestCheckTypeSetElemAttr(cloudAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(cloudAccessResourceName, "service_accounts.#", "1"),
				),
				// The plan will not be empty because JAAS sets the special user "everyone@external"
				// to have access to clouds by default.
				// This behavior is tested in TestAcc_ResourceJaasAccessCloudImportState.
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestAcc_ResourceJaasAccessCloudImportState(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := "localhost"
	access := "can_addmodel"

	resourceName := "juju_jaas_access_cloud.test"

	// Test 0: Test importing works.
	// Note that because JAAS allows the special user "everyone@external" to have add model access
	// to clouds, we check that this user is present when we do an import with an empty config.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:            testAccResourceJaasAccessCloudEmpty(cloudName, access),
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
					errs = append(errs, checker("users.0", "everyone@external"))
					errs = append(errs, checker("users.#", "1"))
					return errors.Join(errs...)
				},
				ImportState:   true,
				ImportStateId: fmt.Sprintf("%s:%s", "cloud-"+cloudName, access),
				ResourceName:  resourceName,
			},
		},
	})
}

func testAccResourceJaasAccessCloud(cloudName, access, user, group, svcAcc string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessCloud",
		`
resource "juju_jaas_group" "test" {
  name = "{{ .Group }}"
}

resource "juju_jaas_access_cloud" "test" {
  cloud_name          = "{{.Cloud}}"
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  groups              = [juju_jaas_group.test.uuid]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"Cloud":  cloudName,
			"Access": access,
			"User":   user,
			"Group":  group,
			"SvcAcc": svcAcc,
		})
}

func testAccResourceJaasAccessCloudEmpty(cloudName, access string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessCloudEmpty",
		`
resource "juju_jaas_access_cloud" "test" {
  cloud_name          = "{{.Cloud}}"
  access              = "{{.Access}}"
}
`, internaltesting.TemplateData{
			"Cloud":  cloudName,
			"Access": access,
		})
}
