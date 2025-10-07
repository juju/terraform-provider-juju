// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
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
			{
				Config: testAccDataSourceStoragePool(modelName, poolName, "tmpfs", dataSourceName, poolAttributes),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_storage_pool."+dataSourceName, "model_uuid", "juju_model."+modelName, "uuid"),
					resource.TestCheckResourceAttr("data.juju_storage_pool."+dataSourceName, "name", poolName),
				),
			},
		},
	})
}

func testAccDataSourceStoragePool(
	modelName,
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
  model_uuid = juju_model.{{.ModelName}}.uuid
}

`, internaltesting.TemplateData{
		"ModelName":           modelName,
		"PoolName":            poolName,
		"StorageProviderName": storageProviderName,
		"PoolAttributes":      poolAttributes,
		"DataSourceName":      dataSourceName,
	})
}
