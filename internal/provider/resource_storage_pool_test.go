// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceStoragePool(t *testing.T) {
	modelName := "test-model"
	poolName := "test-pool"
	storageProviderName := "tmpfs"
	poolAttributes := map[string]string{
		"a": "b",
		"c": "d",
	}

	resourceFullName := "juju_storage_pool." + poolName
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Plan, Apply testing:
			{
				Config: testAccResourceStoragePoolConfig2(modelName, poolName, storageProviderName, poolAttributes),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
					resource.TestCheckResourceAttr(resourceFullName, "name", poolName),
					resource.TestCheckResourceAttr(resourceFullName, "model", modelName),
					resource.TestCheckResourceAttr(resourceFullName, "storageprovider", storageProviderName),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.a", "b"),
					resource.TestCheckResourceAttr(resourceFullName, "attributes.c", "d"),
				),
			},
			// ImportState testing: (Not needed, cannot import storage pools.)
			// RefreshState testing:
		},
	})

}

func testAccResourceStoragePoolConfig2(modelName, poolName, storageProviderName string, poolAttributes map[string]string) string {
	return internaltesting.GetStringFromTemplateWithData("testAccResourceApplicationStorage", `
resource "juju_model" "{{.ModelName}}" {
	name = "{{.ModelName}}"
}

resource "juju_storage_pool" "{{.PoolName}}" {
	name = "{{.PoolName}}"
	model = juju_model.{{.ModelName}}.name
	storageprovider = "{{.StorageProviderName}}"
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
