// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceStoragePool(t *testing.T) {
	modelName := acctest.RandomWithPrefix("test-model")

	poolName := "test-pool"
	storageProviderName := "tmpfs"

	resourceFullName := "juju_storage_pool." + poolName
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create with intentionally incorrect storage provider:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceStoragePoolWithAttributes(modelName, poolName, "rootfs", map[string]string{
					"a": "b",
					"c": "d",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", "rootfs"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.a", "b"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.c", "d"),
				),
			},
			// Update storage provider to correct value and expect RequiresReplace:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionReplace),
					},
				},
				Config: testAccResourceStoragePoolWithAttributes(modelName, poolName, storageProviderName, map[string]string{
					"a": "b",
					"c": "d",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.a", "b"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.c", "d"),
				),
			},
			// Update attributes (in-place) and add a new attribute:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceStoragePoolWithAttributes(modelName, poolName, storageProviderName, map[string]string{
					"a":       "benedict",
					"charlie": "d",
					"alpha":   "delta",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.a", "benedict"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.charlie", "d"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.alpha", "delta"),
				),
			},
			// Remove all attributes:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceStoragePoolNoAttributes(modelName, poolName, storageProviderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
				),
			},
			// Add attributes back from null attributes:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceStoragePoolWithAttributes(modelName, poolName, storageProviderName, map[string]string{
					"alice": "ourtrueuser0",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.alice", "ourtrueuser0"),
				),
			},
		},
	})
}

// Tests that creating a pool with no attributes (nulled in state) works as expected when updated to a value.
func TestAcc_ResourceStoragePool_CreateNoAttributes(t *testing.T) {
	modelName := acctest.RandomWithPrefix("test-model")
	poolName := "test-pool"
	storageProviderName := "tmpfs"

	resourceFullName := "juju_storage_pool." + poolName
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create with pool attributes:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceStoragePoolNoAttributes(modelName, poolName, storageProviderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.%", "0"),
				),
			},
			// Update attributes (in-place) and add a new attribute:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
					},
				},
				Config: testAccResourceStoragePoolWithAttributes(modelName, poolName, storageProviderName, map[string]string{
					"a":       "benedict",
					"charlie": "d",
					"alpha":   "delta",
				}),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.a", "benedict"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.charlie", "d"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.alpha", "delta"),
				),
			},
		},
	})
}

// Tests that creating a pool with no attributes (nulled in state) works as expected when updated to a value.
func TestAcc_ResourceStoragePool_ImportState(t *testing.T) {
	modelName := acctest.RandomWithPrefix("test-model")
	poolName := "test-pool"
	storageProviderName := "tmpfs"

	resourceFullName := "juju_storage_pool." + poolName
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Create with pool attributes:
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceStoragePoolNoAttributes(modelName, poolName, storageProviderName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "storage_provider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.%", "0"),
				),
			},
			{
				ResourceName: resourceFullName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs := s.RootModule().Resources[resourceFullName]
					modelUUID := rs.Primary.Attributes["model_uuid"]
					return fmt.Sprintf("%s:%s", modelUUID, poolName), nil
				},
				ImportStateVerify: true,
			},
		},
	})
}

func testAccResourceStoragePoolWithAttributes(modelName, poolName, storageProviderName string, poolAttributes map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
	name = "{{.ModelName}}"
}

resource "juju_storage_pool" "{{.PoolName}}" {
	name = "{{.PoolName}}"
	model_uuid = juju_model.{{.ModelName}}.uuid
	storage_provider = "{{.StorageProviderName}}"
	attributes = {
	{{- range $key, $value := .PoolAttributes }}
	{{$key}} = "{{$value}}"
	{{- end }}
	}
}

`, internaltesting.TemplateData{
		"ModelName":           modelName,
		"PoolName":            poolName,
		"StorageProviderName": storageProviderName,
		"PoolAttributes":      poolAttributes,
	})
}

func testAccResourceStoragePoolNoAttributes(modelName, poolName, storageProviderName string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
	name = "{{.ModelName}}"
}

resource "juju_storage_pool" "{{.PoolName}}" {
	name = "{{.PoolName}}"
	model_uuid = juju_model.{{.ModelName}}.uuid
	storage_provider = "{{.StorageProviderName}}"
}

`, internaltesting.TemplateData{
		"ModelName":           modelName,
		"PoolName":            poolName,
		"StorageProviderName": storageProviderName,
	})
}
