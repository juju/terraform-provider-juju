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

// This file has bare minimum tests for offer access
// verifying that users, service accounts and groups
// can access an offer. More extensive tests for
// generic jaas access are available in
// resource_access_jaas_model_test.go

func TestAcc_ResourceJaasAccessOffer(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// Resource names
	modelName := acctest.RandomWithPrefix("tf-test-offer")
	offerAccessResourceName := "juju_jaas_access_offer.test"
	groupResourcename := "juju_jaas_group.test"
	roleResourcename := "juju_jaas_role.test"
	accessSuccess := "consumer"
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
	offerRelationF := func(s string) string { return names.NewApplicationOfferTag(s).String() }
	offerCheck := newCheckAttribute(offerAccessResourceName, "offer_url", offerRelationF)
	userTag := names.NewUserTag(user).String()
	svcAccTag := names.NewUserTag(svcAccWithDomain).String()

	// Test 0: Test an invalid access string.
	// Test 1: Test adding a valid set user, group and service account.
	// Test 2: Test importing works.
	// Destroy: Test access is removed.
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckJaasResourceAccess(accessSuccess, &userTag, offerCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, offerCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, roleCheck.tag, offerCheck.tag, false),
			testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, offerCheck.tag, false),
		),
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceJaasAccessOffer(modelName, accessFail, user, group, svcAcc, role),
				ExpectError: regexp.MustCompile(fmt.Sprintf("(?s)unknown.*relation %s", accessFail)),
			},
			{
				Config: testAccResourceJaasAccessOffer(modelName, accessSuccess, user, group, svcAcc, role),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAttributeNotEmpty(groupCheck),
					testAccCheckAttributeNotEmpty(roleCheck),
					testAccCheckAttributeNotEmpty(offerCheck),
					testAccCheckJaasResourceAccess(accessSuccess, &userTag, offerCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, groupCheck.tag, offerCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, &svcAccTag, offerCheck.tag, true),
					testAccCheckJaasResourceAccess(accessSuccess, roleCheck.tag, offerCheck.tag, true),
					resource.TestCheckResourceAttr(offerAccessResourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(offerAccessResourceName, "users.*", user),
					resource.TestCheckResourceAttr(offerAccessResourceName, "users.#", "1"),
					// Wrap this check so that the pointer has deferred evaluation.
					func(s *terraform.State) error {
						return resource.TestCheckTypeSetElemAttr(offerAccessResourceName, "groups.*", *groupCheck.resourceID)(s)
					},
					resource.TestCheckResourceAttr(offerAccessResourceName, "groups.#", "1"),
					resource.TestCheckTypeSetElemAttr(offerAccessResourceName, "service_accounts.*", svcAcc),
					resource.TestCheckResourceAttr(offerAccessResourceName, "service_accounts.#", "1"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      offerAccessResourceName,
			},
		},
	})
}

func testAccResourceJaasAccessOffer(modelName, access, user, group, svcAcc, role string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasAccessoffer",
		`
resource "juju_model" "modelone" {
	name = "{{.ModelName}}"
}

resource "juju_application" "appone" {
	model = juju_model.modelone.name
	name  = "appone"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_offer" "offerone" {
	model            = juju_model.modelone.name
	application_name = juju_application.appone.name
	endpoint         = "sink"
}

resource "juju_jaas_role" "test" {
  name = "{{ .Role }}"
}

resource "juju_jaas_group" "test" {
  name = "{{ .Group }}"
}

resource "juju_jaas_access_offer" "test" {
  offer_url           = juju_offer.offerone.url
  access              = "{{.Access}}"
  users               = ["{{.User}}"]
  groups              = [juju_jaas_group.test.uuid]
  roles              = [juju_jaas_role.test.uuid]
  service_accounts    = ["{{.SvcAcc}}"]
}
`, internaltesting.TemplateData{
			"ModelName": modelName,
			"Access":    access,
			"User":      user,
			"Group":     group,
			"SvcAcc":    svcAcc,
			"Role":      role,
		})
}
