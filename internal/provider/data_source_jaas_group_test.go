// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_DataSourceJAASGroup(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	groupName := acctest.RandomWithPrefix("tf-jaas-group")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceJAASGroup(groupName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_jaas_group.test", "name", groupName),
					resource.TestCheckResourceAttrSet("data.juju_jaas_group.test", "uuid"),
					resource.TestCheckResourceAttrPair("juju_jaas_group.test", "uuid", "data.juju_jaas_group.test", "uuid"),
				),
			},
		},
	})
}

func testAccDataSourceJAASGroup(name string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccDataSourceJAASGroup",
		`
resource "juju_jaas_group" "test" {
	name = "{{ .Name }}"
}

data "juju_jaas_group" "test" {
	name = juju_jaas_group.test.name
}
`, internaltesting.TemplateData{
			"Name": name,
		})
}
