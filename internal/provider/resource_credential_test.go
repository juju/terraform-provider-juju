package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceCredential_Basic(t *testing.T) {
	credentialName := acctest.RandomWithPrefix("tf-test-credential")
	credentialInvalidName := "tf%test_credential"
	authType := "certificate"
	authTypeInvalid := "invalid"
	token := "123abc"

	resourceName := "juju_credential.credential"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceCredential(t, credentialName, authTypeInvalid),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: supported auth-types (.*), \"%s\" not supported", authTypeInvalid)),
			},
			{
				Config:      testAccResourceCredential(t, credentialInvalidName, authType),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: \"%s\" is not a valid credential name", credentialInvalidName)),
			},
			{
				Config: testAccResourceCredential(t, credentialName, authType),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", credentialName),
					resource.TestCheckResourceAttr(resourceName, "auth_type", authType),
				),
			},
			{
				Config: testAccResourceCredentialToken(t, credentialName, authType, token),
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

func testAccResourceCredential(t *testing.T, credentialName string, authType string) string {
	return fmt.Sprintf(`
resource "juju_credential" "credential" {
  name = %q

  cloud {
   name   = "localhost"
  }

  auth_type = "%s"
}`, credentialName, authType)
}

func testAccResourceCredentialToken(t *testing.T, credentialName, authType, token string) string {
	return fmt.Sprintf(`
resource "juju_credential" "credential" {
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
