package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceAccessModel_sdk2_framework_migrate(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	userName2 := acctest.RandomWithPrefix("tfuser")
	userPassword2 := acctest.RandomWithPrefix("tf-test-user")
	modelName1 := "testing1"
	modelName2 := "testing2"
	accessSuccess := "write"
	accessFail := "bogus"

	resourceName := "juju_access_model.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceAccessModel_sdk2_framework_migrate(userName, userPassword, modelName1, accessFail),
				ExpectError: regexp.MustCompile("Error running pre-apply refresh.*"),
			},
			{
				Config: testAccResourceAccessModel_sdk2_framework_migrate(userName, userPassword, modelName1, accessSuccess),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "model", modelName1),
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName1, accessSuccess, userName),
				ResourceName:      resourceName,
			},
			{
				Config: testAccFrameworkResourceAccessModel_sdk2_framework_migrate(t, userName2, userPassword2,
					modelName2, accessSuccess),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckResourceAttr(resourceName, "model", modelName2),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName2),
				),
			},
			{
				Destroy:           true,
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName2, accessSuccess, userName2),
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceAccessModel_sdk2_framework_migrate(userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_user" "this" {
  name = %q
  password = %q
}

resource "juju_model" "this" {
  provider = oldjuju
  name = %q
}

resource "juju_access_model" "test" {
  access = %q
  model = juju_model.this.name
  users = [juju_user.this.name]
}`, userName, userPassword, modelName, access)
}

func testAccFrameworkResourceAccessModel_sdk2_framework_migrate(t *testing.T, userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
provider "juju" {}
provider "oldjuju" {}

resource "juju_user" "test-user" {
  name = %q
  password = %q
}

resource "juju_model" "test-model" {
  provider = oldjuju
  name = %q
}

resource "juju_access_model" "test" {
  provider = juju
  access = %q
  model = juju_model.test-model.name
  users = [juju_user.test-user.name]
}`, userName, userPassword, modelName, access)
}

func TestAcc_ResourceAccessModel_Stable(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
	modelName := "testing"
	access := "write"
	accessFail := "bogus"

	resourceName := "juju_access_model.test"
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
				Config:      testAccResourceAccessModel_Stable(userName, userPassword, modelName, accessFail),
				ExpectError: regexp.MustCompile("Error running pre-apply refresh.*"),
			},
			{
				// (juanmanuel-tirado) For some reason beyond my understanding,
				// this test fails no microk8s on GitHub. If passes in local
				// environments with no additional configurations...
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				Config: testAccResourceAccessModel_Stable(userName, userPassword, modelName, access),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "access", access),
					resource.TestCheckResourceAttr(resourceName, "model", modelName),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
			},
			{
				SkipFunc: func() (bool, error) {
					return testingCloud != LXDCloudTesting, nil
				},
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName, access, userName),
				ResourceName:      resourceName,
			},
		},
	})
}

func testAccResourceAccessModel_Stable(userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
resource "juju_user" "test-user" {
  name = %q
  password = %q
}

resource "juju_model" "test-model" {
  name = %q
}

resource "juju_access_model" "test" {
  access = %q
  model = juju_model.test-model.name
  users = [juju_user.test-user.name]
}`, userName, userPassword, modelName, access)
}
