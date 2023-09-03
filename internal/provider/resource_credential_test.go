// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceCredential_sdk2_framework_migrate(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	credentialInvalidName := "tf%test_credential"
	authType := "certificate"
	authTypeInvalid := "invalid"
	token := "123abc"

	resourceName := "juju_credential.test-credential"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceCredential_sdk2_framework_migrate(t, credentialName, authTypeInvalid),
				ExpectError: regexp.MustCompile(fmt.Sprintf("%q not supported", authTypeInvalid)),
				PreConfig:   func() { testAccPreCheck(t) },
			},
			{
				Config: testAccResourceCredential_sdk2_framework_migrate(t, credentialInvalidName, authType),
				ExpectError: regexp.MustCompile(fmt.Sprintf(".*%q is not\na valid credential name.*",
					credentialInvalidName)),
			},
			{
				Config: testAccResourceCredential_sdk2_framework_migrate(t, credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
				),
			},
			{
				Config: testAccResourceCredentialToken_sdk2_framework_migrate(t, credentialName, authType, token),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
					resource.TestCheckResourceAttr(resourceName, "attributes.token", token),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"attributes.%",
					"attributes.token"},
				ImportStateId: fmt.Sprintf("%s:localhost:false:true", credentialName),
				ResourceName:  resourceName,
			},
		},
	})
}

func testAccResourceCredential_sdk2_framework_migrate(t *testing.T, credentialName string, authType string) string {
	return fmt.Sprintf(`
resource "juju_credential" "test-credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"
}`, credentialName, authType)
}

func testAccResourceCredentialToken_sdk2_framework_migrate(t *testing.T, credentialName, authType, token string) string {
	return fmt.Sprintf(`
resource "juju_credential" "test-credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"

  attributes = {
	token = "%s"
  }
}`, credentialName, authType, token)
}

func TestAcc_ResourceCredential_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	credentialInvalidName := "tf%test_credential"
	authType := "certificate"
	authTypeInvalid := "invalid"
	token := "123abc"

	resourceName := "juju_credential.test-credential"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceCredential_Stable(t, credentialName, authTypeInvalid),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: supported auth-types (.*), \"%s\" not supported", authTypeInvalid)),
			},
			{
				Config:      testAccResourceCredential_Stable(t, credentialInvalidName, authType),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: \"%s\" is not a valid credential name", credentialInvalidName)),
			},
			{
				Config: testAccResourceCredential_Stable(t, credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
				),
			},
			{
				Config: testAccResourceCredentialToken_Stable(t, credentialName, authType, token),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
					resource.TestCheckResourceAttr(resourceName, "attributes.token", token),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"attributes.%",
					"attributes.token"},
				ImportStateId: fmt.Sprintf("%s:localhost:false:true", credentialName),
				ResourceName:  resourceName,
			},
		},
	})
}

func testAccResourceCredential_Stable(t *testing.T, credentialName string, authType string) string {
	return fmt.Sprintf(`
resource "juju_credential" "test-credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"
}`, credentialName, authType)
}

func testAccResourceCredentialToken_Stable(t *testing.T, credentialName, authType, token string) string {
	return fmt.Sprintf(`
resource "juju_credential" "test-credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"

  attributes = {
	token = "%s"
  }
}`, credentialName, authType, token)
}
