// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccListStoragePools_query(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-test-storagepool-list")
	poolName := "test-pool"
	storageProvider := "tmpfs"

	var expectedID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListStoragePoolsResourceConfig(modelName, poolName, storageProvider),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_storage_pool.test"]
					if !ok {
						return fmt.Errorf("not found: juju_storage_pool.test")
					}
					expectedID = rs.Primary.ID
					return nil
				},
			},
			{
				Config: testAccListStoragePools(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_storage_pool.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
					querycheck.ExpectResourceKnownValues(
						"juju_storage_pool.test",
						queryfilter.ByResourceIdentity(map[string]knownvalue.Check{
							"id": knownvalue.StringFunc(func(actual string) error {
								return knownvalue.StringExact(expectedID).CheckValue(actual)
							}),
						}),
						[]querycheck.KnownValueCheck{
							{
								Path:       tfjsonpath.New("name"),
								KnownValue: knownvalue.StringExact(poolName),
							},
							{
								Path:       tfjsonpath.New("storage_provider"),
								KnownValue: knownvalue.StringExact(storageProvider),
							},
							{
								Path:       tfjsonpath.New("attributes").AtMapKey("alpha"),
								KnownValue: knownvalue.StringExact("delta"),
							},
						},
					),
				},
			},
			{
				Config: testAccListStoragePoolExact(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLength("juju_storage_pool.test", 1),
					querycheck.ExpectIdentity("juju_storage_pool.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
				},
			},
		},
	})
}

func testAccListStoragePoolsResourceConfig(modelName, poolName, storageProvider string) string {
	return fmt.Sprintf(`
resource "juju_model" "test" {
	name = %q
}

resource "juju_storage_pool" "test" {
	name             = %q
	model_uuid       = juju_model.test.uuid
	storage_provider = %q
	attributes = {
					"alpha": "delta",
	}
}
`, modelName, poolName, storageProvider)
}

func testAccListStoragePools() string {
	return `
list "juju_storage_pool" "test" {
  provider         = juju
  include_resource = true

  config {
		model_uuid = juju_model.test.uuid
  }
}
`
}

func testAccListStoragePoolExact() string {
	return `
list "juju_storage_pool" "test" {
  provider         = juju
  include_resource = true

  config {
		model_uuid       = juju_model.test.uuid
		name             = juju_storage_pool.test.name
		storage_provider = juju_storage_pool.test.storage_provider
  }
}
`
}
