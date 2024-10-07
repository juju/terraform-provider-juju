package provider

import (
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceKubernetesCloud(t *testing.T) {
	if testingCloud != MicroK8sTesting {
		t.Skip(t.Name() + " only runs with MicroK8s")
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

//func testAccResourceKubernetesCloud(cloudName string, modelName string, config string) string {
//	return internaltesting.GetStringFromTemplateWithData(
//		"testAccResourceSecret",
//		`
//resource "juju_kubernetes_cloud" {{.CloudName}} {
//  name = "{{.CloudName}}"
//  kubernetes_config = {{.Config}}
//}
//
//resource "juju_model" {{.ModelName}} {
//  name = "{{.ModelName}}"
//  credential = juju_kubernetes_cloud.{{.CloudName}}.credential
//  cloud {
//    name   = juju_kubernetes_cloud.{{.CloudName}}.name
//  }
//}
//`, internaltesting.TemplateData{
//			"CloudName": cloudName,
//			"ModelName": modelName,
//			"Config":    config,
//		})
//}

func testAccResourceKubernetesCloudWithoutModel(cloudName string, config string) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccResourceSecret",
		`
resource "juju_kubernetes_cloud" {{.CloudName}} {
  name = "{{.CloudName}}"
  kubernetes_config = {{.Config}}
}
`, internaltesting.TemplateData{
			"CloudName": cloudName,
			"Config":    config,
		})
}
