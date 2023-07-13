package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceAccessModel_sdk2_framework_migrate(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")
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
					resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					resource.TestCheckResourceAttr(resourceName, "model", modelName1),
					resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
				),
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: "0.8.0",
						Source:            "juju/juju",
					},
				},
			},
			{
				Config: testAccFrameworkResourceAccessModel(t, userName, userPassword, modelName2, accessSuccess),
				Check: resource.ComposeTestCheckFunc(
					// resource.TestCheckResourceAttr(resourceName, "access", accessSuccess),
					// resource.TestCheckResourceAttr(resourceName, "model", modelName2),
					// resource.TestCheckTypeSetElemAttr(resourceName, "users.*", userName),
					resource.TestCheckResourceAttr("juju_model.test-model", "name", modelName2),
				),
				ProtoV5ProviderFactories: map[string]func() (tfprotov5.ProviderServer, error){
					"juju": providerserver.NewProtocol5WithError(NewJujuProvider("dev")),
					"oldjuju": func() (tfprotov5.ProviderServer, error) {
						return schema.NewGRPCProviderServer(New("dev")()), nil
					},
				},
			},
			/*
				{

					SkipFunc: func() (bool, error) {
						return testingCloud != LXDCloudTesting, nil
					},

					ImportStateVerify: true,
					ImportState:       true,
					ImportStateId:     fmt.Sprintf("%s:%s:%s", modelName3, accessSuccess, userName),
					ResourceName:      resourceName,
					ProtoV5ProviderFactories: map[string]func() (tfprotov5.ProviderServer, error){
						"juju": providerserver.NewProtocol5WithError(NewJujuProvider("dev")),
						"oldjuju": func() (tfprotov5.ProviderServer, error) {
							return schema.NewGRPCProviderServer(New("dev")()), nil
						},
					},
				},

			*/
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
  provider = oldjuju
  access = %q
  model = juju_model.this.name
  users = [juju_user.this.name]
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

func testAccFrameworkResourceAccessModel(t *testing.T, userName, userPassword, modelName, access string) string {
	return fmt.Sprintf(`
provider "juju" {}
provider "oldjuju" {}

resource "juju_user" "test-user" {
  provider = oldjuju
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
