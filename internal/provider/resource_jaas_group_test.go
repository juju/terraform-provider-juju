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

func TestAcc_ResourceJaasGroup(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	groupName := acctest.RandomWithPrefix("tf-jaas-group")
	newGroupName := acctest.RandomWithPrefix("tf-jaas-group-new")
	resourceName := "juju_jaas_group.test"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckJaasGroupExists(resourceName, false),
		Steps: []resource.TestStep{
			{
				Config: testAccResourceJaasGroup(groupName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", groupName),
					resource.TestCheckResourceAttrSet(resourceName, "uuid"),
					testAccCheckJaasGroupExists(resourceName, true),
				),
			},
			{
				Config: testAccResourceJaasGroup("_invalid group"),
				// Might break if the formatting changes
				ExpectError: regexp.MustCompile("must start with a letter, end with a letter or number"),
			},
			{
				Config: testAccResourceJaasGroup(newGroupName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", newGroupName),
					resource.TestCheckResourceAttrSet(resourceName, "uuid"),
					testAccCheckJaasGroupExists(resourceName, true),
				),
			},
		},
	})
}

func testAccResourceJaasGroup(name string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceJaasGroup",
		`
resource "juju_jaas_group" "test" {
  name = "{{ .Name }}"
}
`, internaltesting.TemplateData{
			"Name": name,
		})
}

// testAccCheckJaasGroupExists returns a function that checks if the group exists if checkExists is true or if it doesn't exist if checkExists is false.
func testAccCheckJaasGroupExists(resourceName string, checkExists bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("Group %q not found", resourceName)
		}

		uuid := rs.Primary.Attributes["uuid"]
		if uuid == "" {
			return errors.New("No group uuid is set")
		}

		_, err := TestClient.Jaas.ReadGroupByUUID(uuid)
		if checkExists && err != nil {
			return fmt.Errorf("Group with uuid %q does not exist", uuid)
		} else if !checkExists && err == nil {
			return fmt.Errorf("Group with uuid %q still exists", uuid)
		}

		return nil
	}
}
