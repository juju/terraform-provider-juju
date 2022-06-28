package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAcc_ResourceIntegration(t *testing.T) {
	t.Skip("resource not yet implemented, remove this once you add your own code")

	resource.UnitTest(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		CheckDestroy:      testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("juju_model.development", "name", regexp.MustCompile("^development")),
					resource.TestMatchResourceAttr("juju_charm.postgres", "charm", regexp.MustCompile("^ch:postgres-k8s")),
					resource.TestMatchResourceAttr("juju_charm.mattermost", "charm", regexp.MustCompile("^ch:mattermost-k8s")),
				),
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	return nil
}

const testAccResourceIntegration = `
resource "juju_model" "development" {
  name = "development"
}

resource "juju_charm" "postgres" {
  model = juju_model.development.id
  charm = "ch:postgres-k8s"
  scale = 3
}

resource "juju_charm" "mattermost" {
  model = juju_model.development.id
  charm = "ch:mattermost-k8s"
  scale = 1
  config = {
    primary_channel = "Town Square"
    license = "My License"
    site_url = "mattermost.dev"
  }
}

resource "juju_integration" "postgres_mattermost" {
  src = juju_charm.postgres.id
  dst = juju_charm.mattermost.id
}
`
