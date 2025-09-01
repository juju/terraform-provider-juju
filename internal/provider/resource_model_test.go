// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/rpc/params"
)

var validUUID = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)

func TestAcc_ResourceModel(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	logLevelInfo := "INFO"
	logLevelDebug := "DEBUG"

	resourceName := "juju_model.model"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelInfo),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelInfo)),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
				),
			},
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				Config: testAccConstraintsModel(modelName, testingCloud.CloudName(), "cores=1 mem=1024M"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "constraints", "cores=1 mem=1024M"),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      resourceName,
			},
		},
	})
}

func TestAcc_ResourceModel_UnsetConfig(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resourceName := "juju_model.this"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
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
					testAccCheckDevelopmentConfigIsUnset("juju_model.this"),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_Minimal(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_UpgradeProvider(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	logLevelDebug := "DEBUG"

	resourceName := "juju_model.model"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestCheckResourceAttr(resourceName, "config.logging-config", fmt.Sprintf("<root>=%s", logLevelDebug)),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceModel(modelName, testingCloud.CloudName(), logLevelDebug),
				PlanOnly:                 true,
			},
		},
	})
}

func TestAcc_ResourceModel_Annotations_Basic(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsModel(modelName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsModel(modelName, "test", "test-update"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test-update"),
				),
			},
			{
				Config: fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q
}`, modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckNoResourceAttr("juju_model.testmodel", "annotations.test"),
				),
			},
		},
	})
}

func TestAcc_ResourceModel_Annotations_DisjointedSet(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAnnotationsModel(modelName, "test", "test"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test", "test"),
				),
			},
			{
				Config: testAccAnnotationsModel(modelName, "test-another", "test-another"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_model.testmodel", "name", modelName),
					resource.TestCheckResourceAttr("juju_model.testmodel", "annotations.test-another", "test-another"),
					resource.TestCheckNoResourceAttr("juju_model.testmodel", "annotations.test"),
				),
			},
		},
	})
}

// TestAcc_ResourceModel_WaitForDelete tests that the model can be deleted and recreated successfully.
// It ensures that the model is properly cleaned up before the next creation attempt.
func TestAcc_ResourceModel_WaitForDelete(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	resourceName := "juju_model.model"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
				),
			},
			{
				Config: " ",
			},
			{
				Config: testAccResourceModel(modelName, testingCloud.CloudName(), "INFO"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", modelName),
					resource.TestMatchResourceAttr(resourceName, "uuid", validUUID),
				),
			},
		},
	})
}

func testAccCheckDevelopmentConfigIsUnset(resourceID string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceID]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceID)
		}
		modelUUID := rs.Primary.Attributes["uuid"]
		if modelUUID == "" {
			return fmt.Errorf("uuid is empty in state")
		}
		conn, err := TestClient.Models.GetConnection(&modelUUID)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()

		// TODO: consider adding to client so we don't expose this layer (even in tests)
		modelconfigClient := modelconfig.NewClient(conn)

		metadata, err := modelconfigClient.ModelGetWithMetadata()
		if err != nil {
			return err
		}

		actual, found := metadata["development"]
		if !found {
			// not set, which is what we want
			return nil
		}
		expected := params.ConfigValue{
			Value:  false,
			Source: "default",
		}
		if actual.Value != expected.Value || actual.Source != expected.Source {
			return fmt.Errorf("expecting 'development' config for model: %s, to be %#v but was: %#v",
				modelUUID, expected, actual)
		}
		return nil
	}
}

func testAccResourceModel(modelName string, cloudName string, logLevel string) string {
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

func testAccConstraintsModel(modelName string, cloudName string, constraints string) string {
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

func testAccAnnotationsModel(modelName string, annotationKey, annotationValue string) string {
	return fmt.Sprintf(`
resource "juju_model" "testmodel" {
  name = %q

  annotations = {
	%q = %q
  }
}`, modelName, annotationKey, annotationValue)
}
