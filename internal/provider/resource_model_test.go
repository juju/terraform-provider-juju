package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_ResourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	logLevelInfo := "INFO"
	logLevelDebug := "DEBUG"

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(t, modelName, logLevelInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.model", "name", modelName),
					resource.TestCheckResourceAttr(
						"juju_model.model", "config.logging-config", fmt.Sprintf("<root>=%s", logLevelInfo),
					),
				),
			},
			{
				Config: testAccResourceModel(t, modelName, logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"juju_model.model", "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug),
					),
				),
			},
		},
	})
}

// TODO: Merge the import step into the main test once Read has been updated to handle extra config attributes
func TestAcc_ResourceModelImport(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModelImport(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.model", "name", modelName),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateId:     modelName,
				ResourceName:      "juju_model.model",
			},
		},
	})
}

func testAccResourceModel(t *testing.T, modelName string, logLevel string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
    name = "localhost"
    region = "localhost"
  }

  config = {
    logging-config = "<root>=%s"
  }
}`, modelName, logLevel)
}

// TODO: This should not be needed when import can be merged into the main test
func testAccResourceModelImport(t *testing.T, modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q
}`, modelName)
}
