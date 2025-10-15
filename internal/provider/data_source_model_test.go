// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-datasource-model-test")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Test 1: Test an invalid config with both uuid and name/owner set.
			{
				Config:      testAccFrameworkDataSourceModelConflicting(modelName),
				ExpectError: regexp.MustCompile(`.*Invalid Attribute Combination.*`),
			},
			// Test 2: Test an invalid config setting only the model name.
			{
				Config:      testAccFrameworkDataSourceModelByNameAndOwner(modelName, ""),
				ExpectError: regexp.MustCompile(`When looking up a model by name, both the name and owner attributes`),
			},
			// Test 3: Create a model and lookup by UUID.
			{
				Config: testAccFrameworkDataSourceModelByUUID(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.test-model", "uuid", "data.juju_model.test-model", "uuid"),
					resource.TestCheckResourceAttrSet("data.juju_model.test-model", "uuid"),
					resource.TestCheckResourceAttr("data.juju_model.test-model", "name", modelName),
					resource.TestCheckResourceAttr("data.juju_model.test-model", "owner", expectedResourceOwner()),
				),
			},
			// Test 4: Create a model and lookup by name and owner.
			{
				Config: testAccFrameworkDataSourceModelByNameAndOwner(modelName, expectedResourceOwner()),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.test-model", "uuid", "data.juju_model.test-model", "uuid"),
					resource.TestCheckResourceAttrSet("data.juju_model.test-model", "uuid"),
					resource.TestCheckResourceAttr("data.juju_model.test-model", "name", modelName),
					resource.TestCheckResourceAttr("data.juju_model.test-model", "owner", expectedResourceOwner()),
				),
			},
		},
	})
}

func testAccFrameworkDataSourceModelByUUID(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
	name = %q
}

data "juju_model" "test-model" {
	uuid = juju_model.test-model.uuid
}`, modelName)
}

func testAccFrameworkDataSourceModelByNameAndOwner(modelName, owner string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
	name = %[1]q
}

data "juju_model" "test-model" {
	name = %[1]q
	owner = %[2]q
}`, modelName, owner)
}

func testAccFrameworkDataSourceModelConflicting(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test-model" {
	name = %[1]q
}

data "juju_model" "test-model" {
	uuid = juju_model.test-model.uuid
	name = %[1]q
}`, modelName)
}
