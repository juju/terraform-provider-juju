package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
	"os"
	"os/exec"
	"regexp"
	"testing"

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
		t.Fatalf("cannot resolve current controller: %s", err)
	}

	cmd := exec.Command("juju", "add-model", modelName)
	// TODO: required - see task #42
	cmd.Env = append(os.Environ(), "JUJU_CONTROLLER="+controllerName)

	err = cmd.Run()
	if err != nil {
		t.Fatalf("error whilst creating model %s: %s", modelName, err)
	}
}
