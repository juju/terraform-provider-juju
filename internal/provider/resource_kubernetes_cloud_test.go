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
				Config: testAccResourceKubernetesCloud(cloudName, "", jaasTest, false, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
			{
				// This step we leverage `removed` to remove the cloud from the state, so we don't issue
				// a `Destroy` on the cloud, which would fail with "cloud is used by 1 model".
				// This is a bug in the provider that should be solved once we wait on model's deletion.
				Config: testAccRemovedResourceKubernetesCloud(cloudName),
			},
		}})
}

func TestAcc_ResourceKubernetesCloudDelete(t *testing.T) {
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
				Config: testAccResourceKubernetesCloud(cloudName, "", jaasTest, false, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
		}})
}

func TestAcc_ResourceKubernetesCloudWithoutServiceAccount(t *testing.T) {
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
				Config: testAccResourceKubernetesCloud(cloudName, "", jaasTest, true, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
			{
				// This step we leverage `removed` to remove the cloud from the state, so we don't issue
				// a `Destroy` on the cloud, which would fail with "cloud is used by 1 model".
				// This is a bug in the provider that should be solved once we wait on model's deletion.
				Config: testAccRemovedResourceKubernetesCloud(cloudName),
			},
		},
	})
}

func TestAcc_ResourceKubernetesCloudUpdate(t *testing.T) {
	// We don't support cloud update in JAAS.
	SkipJAAS(t)
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloud(cloudName, "", false, false, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
				),
			},
			{
				Config: testAccResourceKubernetesCloud(cloudName, "test string", false, false, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName),
					resource.TestMatchResourceAttr("juju_kubernetes_cloud."+cloudName, "kubernetes_config", regexp.MustCompile(".*test string.*")),
				),
			},
			{
				// This step we leverage `removed` to remove the cloud from the state, so we don't issue
				// a `Destroy` on the cloud, which would fail with "cloud is used by 1 model".
				// This is a bug in the provider that should be solved once we wait on model's deletion.
				Config: testAccRemovedResourceKubernetesCloud(cloudName),
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

// testAccResourceKubernetesCloud creates a terraform plan to test juju_kubernetes_cloud.
// stringToAppendToConfig is a string appended as a comment to the microk8s config and it is used to simulate
// a change in the kubernetes config.
func testAccResourceKubernetesCloud(cloudName string, stringToAppendToConfig string, jaasTest bool, noServiceAccount bool, withModel bool) string {
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
 {{ if .NoServiceAccount }}
 skip_service_account_creation = true
 {{ end }}
}

{{ if .WithModel }}
resource "juju_model" "{{.CloudName}}-model" {
 name = "{{.CloudName}}-model"
 cloud {
		name = "{{.CloudName}}"
 }

 credential = juju_kubernetes_cloud.{{.CloudName}}.credential
}
{{ end }}

`, internaltesting.TemplateData{
			"CloudName":              cloudName,
			"StringToAppendToConfig": stringToAppendToConfig,
			"JAASTest":               jaasTest,
			"NoServiceAccount":       noServiceAccount,
			"WithModel":              withModel,
		})
}

func testAccRemovedResourceKubernetesCloud(cloudName string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccRemovedResourceKubernetesCloud",
		`
		removed {
  from = juju_kubernetes_cloud.{{.CloudName}}
  lifecycle {
    destroy = false
  }
}`, internaltesting.TemplateData{
			"CloudName": cloudName,
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
