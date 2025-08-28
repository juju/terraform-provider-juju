// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_DataSourceApplicationLXD_Edge(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-application-test-model")
	applicationName := acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceApplicationLXD(modelName, applicationName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.model", "uuid", "data.juju_application.this", "model_uuid"),
					resource.TestCheckResourceAttr("data.juju_application.this", "name", applicationName),
				),
			},
		},
	})
}

func TestAcc_DataSourceApplicationK8s_Edge(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with MicroK8s")
	}
	modelName := acctest.RandomWithPrefix("tf-datasource-application-test-model")
	applicationName := acctest.RandStringFromCharSet(10, acctest.CharSetAlpha)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceApplicationK8s(modelName, applicationName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.model", "uuid", "data.juju_application.this", "model_uuid"),
					resource.TestCheckResourceAttr("data.juju_application.this", "name", applicationName),
				),
			},
		},
	})
}

func testAccDataSourceApplicationLXD(modelName, applicationName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}

resource "juju_application" "this" {
  name = %q
  model_uuid = juju_model.model.uuid
  trust = true

  charm {
    name     = "ubuntu"
	channel  = "latest/stable"
  }
}

data "juju_application" "this" {
  model_uuid = juju_model.model.uuid
  name = juju_application.this.name

}`, modelName, applicationName)
}

func testAccDataSourceApplicationK8s(modelName, applicationName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}

resource "juju_application" "this" {
  name = %q
  model_uuid = juju_model.model.uuid
  trust = true

  charm {
    name     = "coredns"
	revision = 165
  }
}

data "juju_application" "this" {
  model_uuid = juju_model.model.uuid
  name = juju_application.this.name

}`, modelName, applicationName)
}
