package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceUser_Basic(t *testing.T) {
	userName := "tftestuser"
	userPassword := acctest.RandomWithPrefix("tf-test-user")

	resourceName := "juju_user.user"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceUser(t, userName, userPassword),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", userName),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"config.%",
					"config.logging-config"},
				ImportStateId: userName,
				ResourceName:  resourceName,
			},
		},
	})
}

func testAccResourceUser(t *testing.T, userName, userPassword string) string {
	return fmt.Sprintf(`
resource "juju_user" "user" {
  name = %q
  password = %q

}`, userName, userPassword)
}
