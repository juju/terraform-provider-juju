// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceModel_Edge(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-model-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFrameworkDataSourceModel(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.test-model", "uuid", "data.juju_model.test-model", "uuid"),
					resource.TestCheckResourceAttrSet("data.juju_model.test-model", "uuid"),
				),
			},
		},
	})
}

func testAccFrameworkDataSourceModel(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
	name = %q
}

data "juju_model" "test-model" {
	uuid = juju_model.test-model.uuid
}`, modelName)
}
