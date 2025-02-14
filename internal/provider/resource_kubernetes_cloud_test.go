// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"os"
	"regexp"
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

	jaasTest := false
	if _, ok := os.LookupEnv("IS_JAAS"); ok {
		jaasTest = true
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloudWithoutModel(cloudName, "", jaasTest),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
		},
	})
}

func TestAcc_ResourceKubernetesCloudUpdate(t *testing.T) {
	// We don't support cloud update in JAAS.
	SkipJAAS(t)
	// TODO: This test is not adding model as a resource, which is required.
	// The reason is the race that we (potentially) have in the Juju side.
	// Once the problem is fixed (https://bugs.launchpad.net/juju/+bug/2084448),
	// we should add the model as a resource.
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloudWithoutModel(cloudName, "", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
			{
				Config: testAccResourceKubernetesCloudWithoutModel(cloudName, "test string", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
					resource.TestMatchResourceAttr("juju_kubernetes_cloud."+cloudName, "kubernetes_config", regexp.MustCompile(".*test string.*")),
				),
			},
		},
	})
}

func TestAcc_ResourceKubernetesCloudWithJAASIncompleteConfig(t *testing.T) {
	OnlyTestAgainstJAAS(t)
	// TODO: This test is not adding model as a resource, which is required.
	// The reason in the race that we (potentially) have in the Juju side.
	// Once the problem is fixed (https://bugs.launchpad.net/juju/+bug/2084448),
	// we should add the model as a resource.
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			Config:      testAccResourceKubernetesCloudWithoutParentCloudName(cloudName),
			ExpectError: regexp.MustCompile("Field `parent_cloud_name` must be specified when applying to a JAAS.*"),
		}},
	})
}

// testAccResourceKubernetesCloudWithoutModel creates a terraform plan to test juju_kubernetes_cloud.
// stringToAppendToConfig is a string appended as a comment to the microk8s config and it is used to simulate
// a change in the kubernetes config.
func testAccResourceKubernetesCloudWithoutModel(cloudName string, stringToAppendToConfig string, jaasTest bool) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_kubernetes_cloud" "{{.CloudName}}" {
 name = "{{.CloudName}}"
 {{if .StringToAppendToConfig }}
 kubernetes_config = "${file("~/microk8s-config.yaml")}\n# {{.StringToAppendToConfig}} \n"
 {{ else }}
 kubernetes_config = file("~/microk8s-config.yaml")
 {{ end }}
 {{ if .JAASTest }}
 parent_cloud_name = "lxd"
 parent_cloud_region = "localhost"
 {{ end }}
}
`, internaltesting.TemplateData{
			"CloudName":              cloudName,
			"StringToAppendToConfig": stringToAppendToConfig,
			"JAASTest":               jaasTest,
		})
}

func testAccResourceKubernetesCloudWithoutParentCloudName(cloudName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_kubernetes_cloud" "{{.CloudName}}" {
 name = "{{.CloudName}}"
 kubernetes_config = "test config"
}
`, internaltesting.TemplateData{
			"CloudName": cloudName,
		})
}
