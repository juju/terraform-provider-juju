package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/juju/terraform-provider-juju/version"
)

func TestAcc_ResourceUser_sdk2_framework_migrate(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")

	resourceName := "juju_user.user"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceUser_Migrate(t, userName, userPassword),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", userName),
				),
			},
			{
				ImportStateVerify:       true,
				ImportState:             true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateId:           fmt.Sprintf("user:%s", userName),
				ResourceName:            resourceName,
			},
		},
	})
}

func testAccResourceUser_Migrate(t *testing.T, userName, userPassword string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_user" "user" {
  provider = oldjuju
  name = %q
  password = %q

}`, userName, userPassword)
}

func TestAcc_ResourceUser_Stable(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")

	resourceName := "juju_user.user"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: version.TerraformProviderJujuVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: testAccResourceUser_Stable(t, userName, userPassword),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", userName),
				),
			},
			{
				ImportStateVerify:       true,
				ImportState:             true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateId:           fmt.Sprintf("user:%s", userName),
				ResourceName:            resourceName,
			},
		},
	})
}

func testAccResourceUser_Stable(t *testing.T, userName, userPassword string) string {
	return fmt.Sprintf(`
resource "juju_user" "user" {
  name = %q
  password = %q

}`, userName, userPassword)
}
