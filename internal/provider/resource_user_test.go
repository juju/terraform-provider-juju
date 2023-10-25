// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceUser(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")

	resourceName := "juju_user.user"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceUser(userName, userPassword),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", userName),
				),
				PreConfig: func() { testAccPreCheck(t) },
			},
			{
				Destroy:                 true,
				ImportStateVerify:       true,
				ImportState:             true,
				ImportStateVerifyIgnore: []string{"password"},
				ImportStateId:           fmt.Sprintf("user:%s", userName),
				ResourceName:            resourceName,
			},
		},
	})
}

func testAccResourceUser(userName, userPassword string) string {
	return fmt.Sprintf(`
resource "juju_user" "user" {
  name = %q
  password = %q

}`, userName, userPassword)
}

func TestAcc_ResourceUser_UpgradeProvider(t *testing.T) {
	userName := acctest.RandomWithPrefix("tfuser")
	userPassword := acctest.RandomWithPrefix("tf-test-user")

	resourceName := "juju_user.user"
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
				Config: testAccResourceUser(userName, userPassword),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", userName),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceUser(userName, userPassword),
				PlanOnly:                 true,
			},
		},
	})
}
