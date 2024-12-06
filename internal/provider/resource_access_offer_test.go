// Copyright 2024 Canonical Ltd.
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

func TestAcc_ResourceAccessOffer(t *testing.T) {
	SkipJAAS(t)
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := acctest.RandomWithPrefix("tf-access-model")
	offerURL := fmt.Sprintf("admin/%s.appone", modelName)
	access := "consume"
	newAccess := "admin"
	accessFail := "bogus"

	resourceName := "juju_access_offer.access_appone_endpoint"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{ // Test access name validation
				Config:      testAccResourceAccessOffer(userName, userPassword, modelName, accessFail),
				ExpectError: regexp.MustCompile("Invalid Attribute Value Match.*"),
			},
			{ // Create the resource
				Config: testAccResourceAccessOffer(userName, userPassword, modelName, access),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "offer_url", offerURL),
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{ // Change access from consume to admin
				Config: testAccResourceAccessOffer(userName, userPassword, modelName, newAccess),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "offer_url", offerURL),
					resource.TestCheckResourceAttr(resourceName, "access", newAccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{ // Destroy the resource and validate it can be imported correctly
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", offerURL, newAccess, userName),
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceAccessOffer_ErrorWhenUsedWithJAAS(t *testing.T) {
	OnlyTestAgainstJAAS(t)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessOfferFixedUser(),
				ExpectError: regexp.MustCompile("This resource is not supported with JAAS"),
			},
		},
	})
}

func testAccResourceAccessOfferFixedUser() string {
	return `
resource "juju_access_offer" "test" {
  access = "consume"
  offer_url = "admin/db.mysql"
  users = ["bob"]
}`
}

func testAccResourceAccessOffer(userName, userPassword, modelName, access string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceAccessOffer",
		`
resource "juju_model" "{{.ModelName}}" {
name = "{{.ModelName}}"
}

resource "juju_user" "operator" {
  name = "{{.UserName}}"
  password = "{{.UserPassword}}"
}

resource "juju_application" "appone" {
  name  = "appone"
  model = juju_model.{{.ModelName}}.name

  charm {
    name = "juju-qa-dummy-source"
    base = "ubuntu@22.04"
  }
}

resource "juju_offer" "appone_endpoint" {
  model            = juju_model.{{.ModelName}}.name
  application_name = juju_application.appone.name
  endpoint         = "sink"
}

resource "juju_access_offer" "access_appone_endpoint" {
    offer_url = juju_offer.appone_endpoint.url
    users = [
		juju_user.operator.name,
	]
    access = "{{.Access}}"
}
`, internaltesting.TemplateData{
			"ModelName":    modelName,
			"Access":       access,
			"UserName":     userName,
			"UserPassword": userPassword,
		})
}
