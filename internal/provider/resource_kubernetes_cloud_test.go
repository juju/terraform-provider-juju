// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

func TestAcc_ResourceKubernetesCloud(t *testing.T) {
	// TODO: This test is not adding model as a resource, which is required.
	// The reason in the race that we (potentially) have in the Juju side.
	// Once the problem is fixed (https://bugs.launchpad.net/juju/+bug/2084448),
	// we should add the model as a resource.
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")
	cloudConfig := os.Getenv("MICROK8S_CONFIG")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloudWithoutModel(cloudName, cloudConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
		},
	})
}

func testAccResourceKubernetesCloudWithoutModel(cloudName string, config string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_kubernetes_cloud" "{{.CloudName}}" {
 name = "{{.CloudName}}"
 kubernetes_config = file("~/microk8s-config.yaml")
}
`, internaltesting.TemplateData{
			"CloudName": cloudName,
			"Config":    config,
		})
}
