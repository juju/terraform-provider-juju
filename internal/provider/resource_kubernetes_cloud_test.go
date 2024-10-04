package provider

import (
	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func getFakeCloudConfig() string {
	return `<<-EOT
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ZmFrZS1jZXJ0aWZpY2F0ZS1hdXRob3JpdHktZGF0YQ==
    server: https://10.172.195.202:16443
  name: microk8s-cluster
contexts:
- context:
    cluster: microk8s-cluster
    user: admin
  name: fake-cloud-context
current-context: fake-cloud-context
kind: Config
preferences: {}
users:
- name: admin
  user:
    client-certificate-data: ZmFrZS1jbGllbnQtY2VydGlmaWNhdGUtZGF0YQ==
    client-key-data: ZmFrZS1jbGllbnQta2V5LWRhdGE=
EOT
`
}

func TestAcc_ResourceKubernetesCloud(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	cloudName := acctest.RandomWithPrefix("tf-test-k8scloud")

	cloudConfig := os.Getenv("MICROK8S_CONFIG")

	//Debug print plan
	t.Logf("Plan: %s", testAccResourceKubernetesCloud(cloudName, cloudConfig))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceKubernetesCloud(cloudName, cloudConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_kubernetes_cloud."+cloudName, "name", cloudName)),
			},
		},
	})
}

func testAccResourceKubernetesCloud(cloudName string, config string) string {
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
