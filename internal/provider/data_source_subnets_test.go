// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcc_DataSourceSubnets(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := "tf-datasource-subnets-test-model"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceSubnets(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_subnets.this", "model_uuid", "juju_model.this", "uuid"),
					testCheckMapNotEmpty("data.juju_subnets.this", "subnets"),
				),
			},
		},
	})
}

func testCheckMapNotEmpty(resourceName, attr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found", resourceName)
		}

		count, ok := rs.Primary.Attributes[attr+".%"]
		if !ok || count == "0" {
			return fmt.Errorf("%s is empty", attr)
		}

		return nil
	}
}

func testAccDataSourceSubnets(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

data "juju_subnets" "this" {
  model_uuid = juju_model.this.uuid
}
`, modelName)
}
