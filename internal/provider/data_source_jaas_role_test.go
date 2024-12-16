// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_DataSourceJAASRole(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	roleName := acctest.RandomWithPrefix("tf-jaas-role")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceJAASRole(roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_jaas_role.test", "name", roleName),
					resource.TestCheckResourceAttrSet("data.juju_jaas_role.test", "uuid"),
					resource.TestCheckResourceAttrPair("juju_jaas_role.test", "uuid", "data.juju_jaas_role.test", "uuid"),
				),
			},
		},
	})
}

func testAccDataSourceJAASRole(name string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccDataSourceJAASRole",
		`
resource "juju_jaas_role" "test" {
	name = "{{ .Name }}"
}

data "juju_jaas_role" "test" {
	name = juju_jaas_role.test.name
}
`, internaltesting.TemplateData{
			"Name": name,
		})
}
