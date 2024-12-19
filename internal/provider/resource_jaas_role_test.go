// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceJaasRole(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	roleName := acctest.RandomWithPrefix("tf-jaas-role")
	newRoleName := acctest.RandomWithPrefix("tf-jaas-role-new")
	resourceName := "juju_jaas_role.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckJaasRoleExists(resourceName, false),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasRole(roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", roleName),
					resource.TestCheckResourceAttrSet(resourceName, "uuid"),
					testAccCheckJaasRoleExists(resourceName, true),
				),
			},
			{
				Config: testAccResourceJaasRole("_invalid role"),
				// Might break if the formatting changes
				ExpectError: regexp.MustCompile("must start with a letter, end with a letter or number"),
			},
			{
				Config: testAccResourceJaasRole(newRoleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", newRoleName),
					resource.TestCheckResourceAttrSet(resourceName, "uuid"),
					testAccCheckJaasRoleExists(resourceName, true),
				),
			},
		},
	})
}

func testAccResourceJaasRole(name string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasRole",
		`
resource "juju_jaas_role" "test" {
  name = "{{ .Name }}"
}
`, internaltesting.TemplateData{
			"Name": name,
		})
}

// testAccCheckJaasRoleExists returns a function that checks if the role exists if checkExists is true or if it doesn't exist if checkExists is false.
func testAccCheckJaasRoleExists(resourceName string, checkExists bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Role %q not found", resourceName)
		}

		uuid := rs.Primary.Attributes["uuid"]
		if uuid == "" {
			return errors.New("No role uuid is set")
		}

		_, err := TestClient.Jaas.ReadRoleByUUID(uuid)
		if checkExists && err != nil {
			return fmt.Errorf("Role with uuid %q does not exist", uuid)
		} else if !checkExists && err == nil {
			return fmt.Errorf("Role with uuid %q still exists", uuid)
		}

		return nil
	}
}
