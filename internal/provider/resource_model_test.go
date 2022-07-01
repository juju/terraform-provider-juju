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
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"config.%",
					"config.logging-config"},
				ImportStateId: modelName,
				ResourceName:  "juju_model.model",
			},
		},
	})
}

func testAccResourceModel(t *testing.T, modelName string, logLevel string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
    name   = "localhost"
    region = "localhost"
  }

  config = {
    logging-config = "<root>=%s"
  }
}`, modelName, logLevel)
}
