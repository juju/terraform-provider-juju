// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceKubernetesCloud(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")
	modelName := acctest.RandomWithPrefix("tf-test-model")
	cloudConfig := os.Getenv("MICROK8S_CONFIG")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloud(cloudName, modelName, cloudConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "model", modelName),
				),
			},
		},
	})
}

func testAccResourceKubernetesCloud(cloudName string, modelName string, config string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`


resource "juju_kubernetes_cloud" "tf-test-k8scloud" {
 name = "{{.CloudName}}"
 kubernetes_config = {{.Config}}
}

resource "juju_model" {{.ModelName}} {
 name = "{{.ModelName}}"
 credential = juju_kubernetes_cloud.tf-test-k8scloud.credential
 cloud {
   name   = juju_kubernetes_cloud.tf-test-k8scloud.name
 }
}
`, internaltesting.TemplateData{
			"CloudName": cloudName,
			"ModelName": modelName,
			"Config":    config,
		})
}
