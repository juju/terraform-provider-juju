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
	AdminUserName := acctest.RandomWithPrefix("tfadminuser")
	ConsumeUserName := acctest.RandomWithPrefix("tfconsumeuser")
	ReadUserName := acctest.RandomWithPrefix("tfreaduser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := acctest.RandomWithPrefix("tf-access-model")
	offerURL := fmt.Sprintf("admin/%s.appone", modelName)

	resourceName := "juju_access_offer.access_appone_endpoint"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{ // Test username overlap validation
				Config:      testAccResourceAccessOffer(AdminUserName, ConsumeUserName, ReadUserName, "admin", "admin", "", userPassword, modelName),
				ExpectError: regexp.MustCompile("appears in.*"),
			},
			{ // Create the resource with user as admin
				Config: testAccResourceAccessOffer(AdminUserName, ConsumeUserName, ReadUserName, "admin", "consume", "read", userPassword, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "offer_url", offerURL),
					resource.TestCheckTypeSetElemAttr(resourceName, "admin.*", AdminUserName),
					resource.TestCheckTypeSetElemAttr(resourceName, "consume.*", ConsumeUserName),
					resource.TestCheckTypeSetElemAttr(resourceName, "read.*", ReadUserName),
				),
			},
			{ // Change Admin to Consume and Consume to Admin
				Config: testAccResourceAccessOffer(AdminUserName, ConsumeUserName, ReadUserName, "consume", "admin", "read", userPassword, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "offer_url", offerURL),
					resource.TestCheckTypeSetElemAttr(resourceName, "admin.*", ConsumeUserName),
					resource.TestCheckTypeSetElemAttr(resourceName, "consume.*", AdminUserName),
					resource.TestCheckTypeSetElemAttr(resourceName, "read.*", ReadUserName),
				),
			},
			{ // Remove user from read permission
				Config: testAccResourceAccessOffer(AdminUserName, ConsumeUserName, ReadUserName, "consume", "admin", "", userPassword, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "offer_url", offerURL),
					resource.TestCheckTypeSetElemAttr(resourceName, "admin.*", ConsumeUserName),
					resource.TestCheckTypeSetElemAttr(resourceName, "consume.*", AdminUserName),
					resource.TestCheckNoResourceAttr(resourceName, "read.*"),
				),
			},
			{ // Destroy the resource and validate it can be imported correctly
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     offerURL,
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

func TestAcc_ResourceAccessOffer_ErrorWhenUsedWithAdmin(t *testing.T) {
	SkipJAAS(t)

	modelNameAdminTest := acctest.RandomWithPrefix("tf-access-admin-model")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{ // Test username admin validation
				Config:      testAccResourceAccessOfferAdminUser(modelNameAdminTest),
				ExpectError: regexp.MustCompile("user admin is not allowed.*"),
			},
		},
	})
}

func testAccResourceAccessOfferFixedUser() string {
	return `
resource "juju_access_offer" "test" {
  offer_url = "admin/db.mysql"
  admin = ["bob"]
}`
}

func testAccResourceAccessOfferAdminUser(modelName string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceAccessOfferAdminUser", `
resource "juju_model" "{{.ModelName}}" {
name = "{{.ModelName}}"
}

resource "juju_application" "appone" {
  name  = "appone"
  model_uuid = juju_model.{{.ModelName}}.uuid

  charm {
    name = "juju-qa-dummy-source"
    base = "ubuntu@22.04"
  }
}

resource "juju_offer" "appone_endpoint" {
  model            = juju_model.{{.ModelName}}.name
  application_name = juju_application.appone.name
  endpoints         = ["sink"]
}

resource "juju_access_offer" "test" {
  offer_url = juju_offer.appone_endpoint.url
  admin = ["admin"]
}`, internaltesting.TemplateData{
		"ModelName": modelName})
}

func testAccResourceAccessOffer(AdminUserName, ConsumeUserName, ReadUserName, OfferAdmin, OfferConsume, OfferRead, userPassword, modelName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceAccessOffer",
		`
resource "juju_model" "{{.ModelName}}" {
name = "{{.ModelName}}"
}

{{- if ne .AdminUserName "" }}
resource "juju_user" "admin_operator" {
  name = "{{.AdminUserName}}"
  password = "{{.UserPassword}}"
}
{{- end }}

{{- if ne .ConsumeUserName "" }}
resource "juju_user" "consume_operator" {
  name = "{{.ConsumeUserName}}"
  password = "{{.UserPassword}}"
}
{{- end }}

{{- if ne .ReadUserName "" }}
resource "juju_user" "read_operator" {
  name = "{{.ReadUserName}}"
  password = "{{.UserPassword}}"
}
{{- end }}

resource "juju_application" "appone" {
  name  = "appone"
  model_uuid = juju_model.{{.ModelName}}.uuid

  charm {
    name = "juju-qa-dummy-source"
    base = "ubuntu@22.04"
  }
}

resource "juju_offer" "appone_endpoint" {
  model            = juju_model.{{.ModelName}}.name
  application_name = juju_application.appone.name
  endpoints         = ["sink"]
}

resource "juju_access_offer" "access_appone_endpoint" {
    offer_url = juju_offer.appone_endpoint.url
	{{- if ne .OfferAdmin "" }}
    admin = [
		juju_user.{{.OfferAdmin}}_operator.name,
	]
	{{- end }}
	{{- if ne .OfferConsume "" }}
	consume = [
		juju_user.{{.OfferConsume}}_operator.name,
	]
	{{- end }}
	{{- if ne .OfferRead "" }}
	read = [
		juju_user.{{.OfferRead}}_operator.name,
	]
	{{- end }}
}
`, internaltesting.TemplateData{
			"ModelName":       modelName,
			"AdminUserName":   AdminUserName,
			"ConsumeUserName": ConsumeUserName,
			"ReadUserName":    ReadUserName,
			"OfferAdmin":      OfferAdmin,
			"OfferConsume":    OfferConsume,
			"OfferRead":       OfferRead,
			"UserPassword":    userPassword,
		})
}
