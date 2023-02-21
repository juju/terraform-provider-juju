package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceAccessModel_Basic(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := "testing"
	access := "write"
	accessFail := "bogus"

	resourceName := "juju_access_model.test"
	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessModel(t, userName, userPassword, modelName, accessFail),
				ExpectError: regexp.MustCompile("Error running pre-apply refresh.*"),
			},
			{
				Config: testAccResourceAccessModel(t, userName, userPassword, modelName, access),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName, access, userName),
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceAccessModel(t *testing.T, userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
resource "juju_user" "this" {
  name = %q
  password = %q
}

resource "juju_model" "this" {
  name = %q
}

resource "juju_access_model" "test" {
  access = %q
  model = juju_model.this.name
  users = [juju_user.this.name]
}`, userName, userPassword, modelName, access)
}
