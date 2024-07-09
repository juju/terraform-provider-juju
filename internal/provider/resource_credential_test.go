// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceCredential(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	authType := "certificate"
	token := "123abc"

	resourceName := "juju_credential.test-credential"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCredential(credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
				),
			},
			{
				Config: testAccResourceCredentialToken(credentialName, authType, token),
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

func TestAcc_ResourceCredential_UpgradeProvider(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	authType := "certificate"

	resourceName := "juju_credential.test-credential"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },

		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceCredential(credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceCredential(credentialName, authType),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccResourceCredential(credentialName string, authType string) string {
	return fmt.Sprintf(`
resource "juju_credential" "test-credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"
}`, credentialName, authType)
}

func testAccResourceCredentialToken(credentialName, authType, token string) string {
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
