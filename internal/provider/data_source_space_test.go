// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceSpace(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	testAccPreCheck(t)

	modelName := acctest.RandomWithPrefix("tf-datasource-space-test-model")
	spaceName := acctest.RandomWithPrefix("tf-datasource-space")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceSpace(modelName, spaceName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_space.this", "model_uuid", "juju_model.this", "uuid"),
					resource.TestCheckResourceAttrPair("data.juju_space.this", "name", "juju_space.this", "name"),
					resource.TestCheckResourceAttrSet("data.juju_space.this", "id"),
				),
			},
		},
	})
}

func testAccDataSourceSpace(modelName, spaceName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_space" "this" {
  model_uuid = juju_model.this.uuid
  name       = %q
}

data "juju_space" "this" {
  model_uuid = juju_model.this.uuid
  name       = juju_space.this.name
}
`, modelName, spaceName)
}
