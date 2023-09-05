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

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccFrameworkDataSourceModel(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_model.test-model", "name", modelName),
					resource.TestCheckResourceAttrSet("data.juju_model.test-model", "uuid"),
				),
			},
		},
	})
}

func TestAcc_DataSourceModel_Stable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-model-test")

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
				Config: testAccFrameworkDataSourceModel(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_model.test-model", "name", modelName),
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
	name = juju_model.test-model.name
}`, modelName)
}
