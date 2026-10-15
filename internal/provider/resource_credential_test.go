// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/juju/juju/rpc/params"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceCredential(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	authType := "certificate"
	token := "123abc"

	resourceName := "juju_credential.test-credential"
	// Adding a localhost credential makes Juju's default credential ambiguous
	// for concurrent tests that create models without an explicit credential.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCredential(credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
				),
			},
			{
				Config: testAccResourceCredentialToken(credentialName, authType, token),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
					resource.TestCheckResourceAttr(resourceName, "attributes.token", token),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
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

func TestAcc_ResourceCredential_RecreateAfterExternalDeletion(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	authType := "certificate"
	resourceName := "juju_credential.test-credential"
	config := testAccResourceCredential(credentialName, authType)

	// Keep the extra localhost credential isolated from model-creation tests.
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
				),
			},
			{
				PreConfig: func() {
					err := TestClient.Credentials.DestroyCredential(t.Context(), juju.DestroyCredentialInput{
						CloudName:            "localhost",
						Name:                 credentialName,
						ControllerCredential: true,
					})
					if err != nil {
						t.Fatalf("failed to delete credential directly from controller: %v", err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionCreate),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
				),
			},
		},
	})
}

func TestAcc_ResourceCredential_ErrorWhenAuthenticatedUserChanges(t *testing.T) {
	SkipJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	resourceName := "juju_credential.test-credential"
	ownerConfig := testAccResourceCredentialWithUser(credentialName, userName, userPassword, false)
	otherUserConfig := testAccResourceCredentialWithUser(credentialName, userName, userPassword, true)

	// Keep the extra localhost credential isolated from model-creation tests.
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			client := TestClient
			_, err := client.Users.CreateUser(t.Context(), juju.CreateUserInput{
				Name: userName, Password: userPassword,
			})
			if err != nil {
				t.Fatalf("failed to create test user: %v", err)
			}
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if t.Failed() {
					err := client.Credentials.DestroyCredential(ctx, juju.DestroyCredentialInput{
						CloudName: "localhost", Name: credentialName, ControllerCredential: true,
					})
					if err != nil && !params.IsCodeNotFound(err) {
						t.Errorf("failed to clean up test credential: %v", err)
					}
				}
				if err := client.Users.DestroyUser(ctx, juju.DestroyUserInput{Name: userName}); err != nil {
					t.Errorf("failed to clean up test user: %v", err)
				}
			})
		},
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: ownerConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
				),
			},
			{
				Config:      otherUserConfig,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`belongs\s+to\s+Juju\s+user`),
			},
			{
				Config:   ownerConfig,
				PlanOnly: true,
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
	// Keep the extra localhost credential isolated from model-creation tests.
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
					// Once TestProviderStableVersion is released, the "owner" attribute
					// will be set by default and this test will need to be updated.
					resource.TestCheckNoResourceAttr(resourceName, "owner"),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceCredential(credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "id", fmt.Sprintf("%s:localhost:false:true", credentialName)),
					resource.TestCheckResourceAttrSet(resourceName, "owner"),
				),
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

// testAccResourceCredentialWithUser switches the provider between the original
// environment user and the user created in PreCheck. User management stays outside
// Terraform so only one provider configuration is active during each test step.
func testAccResourceCredentialWithUser(credentialName, userName, userPassword string, switchUser bool) string {
	authentication := ""
	if switchUser {
		authentication = fmt.Sprintf("username = %q\n  password = %q", userName, userPassword)
	}
	return fmt.Sprintf(`
provider "juju" {
  %s
}

resource "juju_credential" "test-credential" {
  name = %q

  cloud {
    name = "localhost"
  }

  auth_type = "certificate"

}
`, authentication, credentialName)
}
