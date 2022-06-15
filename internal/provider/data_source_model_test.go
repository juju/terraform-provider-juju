package provider

import (
	"fmt"
	"os/exec"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAcc_DataSourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.Test(t, resource.TestCase{
		PreCheck:          func() { testAccPreCheck(t) },
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceModel(t, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr(
						"data.juju_model.model", "name", regexp.MustCompile("^"+modelName+"$")),
				),
			},
		},
		CheckDestroy: testAccDataSourceModelDestroy,
	})
}

func testAccDataSourceModel(t *testing.T, modelName string) string {
	// TODO: required until we can use a resource to create a model
	addModel(t, modelName)

	return fmt.Sprintf(`
data "juju_model" "model" {
  name = %q
}`, modelName)
}

// This function destroys the model created for this test
func testAccDataSourceModelDestroy(s *terraform.State) error {

	for _, rs := range s.RootModule().Resources {
		err := destroyModel(rs.Primary.Attributes["name"])

		if err != nil {
			return err
		}
	}

	return nil
}

// addModel adds a model using the Juju command-line
//
// This function will be removed once we can support creating a
// model resource.
func addModel(t *testing.T, modelName string) {
	store := modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}

	controllerName, err := store.CurrentController()
	if err != nil {
		t.Logf("warning: %s", controllerName)
		return
	}

	cmd := exec.Command("juju", "add-model", "--no-switch", modelName)

	err = cmd.Run()
	if err != nil {
		t.Fatalf("error whilst creating model %s: %s", modelName, err)
	}
}

// destroyModel destroys a model using the Juju command-line
//
// This function will be removed once we can support destroying a
// model resource.
func destroyModel(modelName string) error {
	store := modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}

	controllerName, err := store.CurrentController()
	if err != nil {
		return fmt.Errorf("warning: %s", controllerName)
	}

	cmd := exec.Command("juju", "destroy-model", "-y", modelName)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error whilst destroying model %s: %s", modelName, err)
	}

	return nil
}
