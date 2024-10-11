// Copyright 2024 Canonical Ltd.
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
	cloudName := acctest.RandomWithPrefix("test-k8scloud")
	modelName := "test-model"
	userName := os.Getenv("USER")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloud(cloudName, modelName, userName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud.test-k8scloud", "name", cloudName),
					resource.TestCheckResourceAttr("juju_model.test-model", "name", modelName),
				),
			},
		},
	})
}

func testAccResourceKubernetesCloud(cloudName string, modelName string, userName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceKubernetesCloud",
		`
resource "juju_kubernetes_cloud" "test-k8scloud" {
 name = "{{.CloudName}}"
 kubernetes_config = file("/home/{{.UserName}}/microk8s-config.yaml")
}

resource "juju_model" "test-model" {
 name = "{{.ModelName}}"
 credential = juju_kubernetes_cloud.test-k8scloud.credential
 cloud {
   name   = juju_kubernetes_cloud.test-k8scloud.name
 }
}
`, internaltesting.TemplateData{
			"CloudName": cloudName,
			"ModelName": modelName,
			"UserName":  userName,
		})
}
