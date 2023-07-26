package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/rpc/params"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceModel_sdk2_framework_migrate(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	modelInvalidName := acctest.RandomWithPrefix("tf_test_model")
	logLevelInfo := "INFO"
	logLevelDebug := "DEBUG"

	resourceName := "juju_model.model"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceModelMigrate(t, modelInvalidName, testingCloud.CloudName(), logLevelInfo),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: \"%s\" is not a valid name: model names may only contain lowercase letters, digits and hyphens", modelInvalidName)),
			},
			{
				Config: testAccResourceModelMigrate(t, modelName, testingCloud.CloudName(), logLevelInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelInfo)),
				),
			},
			{
				Config: testAccResourceModelMigrate(t, modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				Config: testAccConstraintsModelMigrate(t, modelName, testingCloud.CloudName(), "cores=1 mem=1024M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "constraints", "cores=1 mem=1024M"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"config.%",
					"config.logging-config"},
				ImportStateId: modelName,
				ResourceName:  resourceName,
			},
		},
	})
}

func TestAcc_ResourceModel_UnsetConfig_sdk2_framework_migrate(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resourceName := "juju_model.this"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: muxProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "this" {
  provider = oldjuju
  name = %q

  config = {
	development = true
  }
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.development", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
provider oldjuju {}
resource "juju_model" "this" {
  provider = oldjuju
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnsetMigrate(modelName),
				),
			},
		},
	})
}

func testAccCheckDevelopmentConfigIsUnsetMigrate(modelName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := Provider.Meta().(*juju.Client)

		uuid, err := client.Models.ResolveModelUUID(modelName)
		if err != nil {
			return err
		}

		conn, err := client.Models.GetConnection(&uuid)
		if err != nil {
			return err
		}

		// TODO: consider adding to client so we don't expose this layer (even in tests)
		modelconfigClient := modelconfig.NewClient(conn)
		defer modelconfigClient.Close()

		metadata, err := modelconfigClient.ModelGetWithMetadata()
		if err != nil {
			return err
		}

		for k, actual := range metadata {
			if k == "development" {
				expected := params.ConfigValue{
					Value:  false,
					Source: "default",
				}

				if actual.Value != expected.Value || actual.Source != expected.Source {
					return fmt.Errorf("expecting 'development' config for model: %s (%s), to be %#v but was: %#v",
						modelName, uuid, expected, actual)
				}
			}
		}
		return nil
	}
}

func testAccResourceModelMigrate(t *testing.T, modelName string, cloudName string, logLevel string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "model" {
  provider = oldjuju
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  config = {
    logging-config = "<root>=%s"
  }
}`, modelName, cloudName, logLevel)
}

func testAccConstraintsModelMigrate(t *testing.T, modelName string, cloudName string, constraints string) string {
	return fmt.Sprintf(`
provider oldjuju {}

resource "juju_model" "model" {
  provider = oldjuju
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  constraints = "%s"
}`, modelName, cloudName, constraints)
}

func TestAcc_ResourceModel_Stable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	modelInvalidName := acctest.RandomWithPrefix("tf_test_model")
	logLevelInfo := "INFO"
	logLevelDebug := "DEBUG"

	resourceName := "juju_model.model"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				// Mind that ExpectError should be the first step
				// "When tests have an ExpectError[...]; this results in any previous state being cleared. "
				// https://github.com/hashicorp/terraform-plugin-sdk/issues/118
				Config:      testAccResourceModelStable(t, modelInvalidName, testingCloud.CloudName(), logLevelInfo),
				ExpectError: regexp.MustCompile(fmt.Sprintf("Error: \"%s\" is not a valid name: model names may only contain lowercase letters, digits and hyphens", modelInvalidName)),
			},
			{
				Config: testAccResourceModelStable(t, modelName, testingCloud.CloudName(), logLevelInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelInfo)),
				),
			},
			{
				Config: testAccResourceModelStable(t, modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				Config: testAccConstraintsModelStable(t, modelName, testingCloud.CloudName(), "cores=1 mem=1024M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "constraints", "cores=1 mem=1024M"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ImportStateVerifyIgnore: []string{
					"config.%",
					"config.logging-config"},
				ImportStateId: modelName,
				ResourceName:  resourceName,
			},
		},
	})
}

func TestAcc_ResourceModel_UnsetConfigStable(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resourceName := "juju_model.this"
	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q

  config = {
	development = true
  }
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.development", "true"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckNoResourceAttr(resourceName, "config.development"),
					testAccCheckDevelopmentConfigIsUnsetStable(modelName),
				),
			},
		},
	})
}

func testAccCheckDevelopmentConfigIsUnsetStable(modelName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		client := Provider.Meta().(*juju.Client)

		uuid, err := client.Models.ResolveModelUUID(modelName)
		if err != nil {
			return err
		}

		conn, err := client.Models.GetConnection(&uuid)
		if err != nil {
			return err
		}

		// TODO: consider adding to client so we don't expose this layer (even in tests)
		modelconfigClient := modelconfig.NewClient(conn)
		defer modelconfigClient.Close()

		metadata, err := modelconfigClient.ModelGetWithMetadata()
		if err != nil {
			return err
		}

		for k, actual := range metadata {
			if k == "development" {
				expected := params.ConfigValue{
					Value:  false,
					Source: "default",
				}

				if actual.Value != expected.Value || actual.Source != expected.Source {
					return fmt.Errorf("expecting 'development' config for model: %s (%s), to be %#v but was: %#v",
						modelName, uuid, expected, actual)
				}
			}
		}
		return nil
	}
}

func testAccResourceModelStable(t *testing.T, modelName string, cloudName string, logLevel string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  config = {
    logging-config = "<root>=%s"
  }
}`, modelName, cloudName, logLevel)
}

func testAccConstraintsModelStable(t *testing.T, modelName string, cloudName string, constraints string) string {
	return fmt.Sprintf(`
resource "juju_model" "model" {
  name = %q

  cloud {
   name   = %q
   region = "localhost"
  }

  constraints = "%s"
}`, modelName, cloudName, constraints)
}
