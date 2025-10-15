// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_DataSourceStoragePool(t *testing.T) {
	// Rootfs, tmpfs and loop are currently not "listable" on k8s (3.6.9) when listing them.
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := acctest.RandomWithPrefix("tf-datasource-storage-pool-test-model")
	poolName := fmt.Sprintf("tf-datasource-storage-pool-test%d", acctest.RandInt())
	poolAttributes := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	dataSourceName := acctest.RandomWithPrefix("tf-datasource-storage-pool")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Lookup by model UUID
			{
				Config: testAccDataSourceStoragePoolByModelUUID(true, false, modelName, "", poolName, "tmpfs", dataSourceName, poolAttributes),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_storage_pool."+dataSourceName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("data.juju_storage_pool."+dataSourceName, "name", poolName),
				),
			},
			// Lookup by model name + owner
			{
				Config: testAccDataSourceStoragePoolByModelUUID(false, true, modelName, expectedResourceOwner(), poolName, "tmpfs", dataSourceName, poolAttributes),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_storage_pool."+dataSourceName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("data.juju_storage_pool."+dataSourceName, "name", poolName),
				),
			},
			// Lookup with only model name set, no owner (fail)
			{
				Config:      testAccDataSourceStoragePoolByModelUUID(false, true, modelName, "", poolName, "tmpfs", dataSourceName, poolAttributes),
				ExpectError: regexp.MustCompile(`When looking up a model by name, both the name and owner attributes`),
			},
		},
	})
}

func testAccDataSourceStoragePoolByModelUUID(
	lookupByModelUUID bool,
	lookupByModelName bool,
	modelName,
	ownerName,
	poolName,
	storageProviderName,
	dataSourceName string,
	poolAttributes map[string]string) string {
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

data "juju_storage_pool" "{{.DataSourceName}}" {
  name       = juju_storage_pool.{{.PoolName}}.name

{{- if .LookupByModelUUID}}
  model_uuid = juju_model.{{.ModelName}}.uuid
{{- end }}

{{ if .LookupByModelName}}
  model_name = juju_model.{{.ModelName}}.name
  model_owner = "{{.ModelOwner}}"
{{- end }}
}

`, internaltesting.TemplateData{
		"LookupByModelUUID":   lookupByModelUUID,
		"LookupByModelName":   lookupByModelName,
		"ModelName":           modelName,
		"ModelOwner":          ownerName,
		"PoolName":            poolName,
		"StorageProviderName": storageProviderName,
		"PoolAttributes":      poolAttributes,
		"DataSourceName":      dataSourceName,
	})
}
