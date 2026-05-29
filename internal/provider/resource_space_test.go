// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	jujuerrors "github.com/juju/errors"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceSpace_CreateImportAndReplace(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	spaceName := "tf-space-a"
	updatedSpaceName := "tf-space-b"

	resourceFullName := "juju_space.this"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
					},
				},
				Config: testAccResourceSpace(modelName, spaceName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model.this", "uuid"),
					resource.TestCheckResourceAttr(resourceFullName, "name", spaceName),
					resource.TestCheckResourceAttrSet(resourceFullName, "id"),
				),
			},
			{
				ResourceName: resourceFullName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceFullName]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					modelUUID := rs.Primary.Attributes["model_uuid"]
					if modelUUID == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:%s", modelUUID, spaceName), nil
				},
				ImportStateVerify: true,
			},
			{
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						ExpectRecreatedResource(resourceFullName),
					},
				},
				Config: testAccResourceSpace(modelName, updatedSpaceName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceFullName, "name", updatedSpaceName),
				),
			},
			{
				Config: testAccResourceSpaceModelOnly(modelName),
				Check:  testAccCheckSpaceAbsent(spaceName),
			},
		},
	})
}

func TestAcc_ResourceSpace_AlphaImportRejected(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")
	spaceName := "tf-space-a"
	resourceFullName := "juju_space.this"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceSpace(modelName, spaceName),
			},
			{
				ResourceName: resourceFullName,
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources[resourceFullName]
					if !ok {
						return "", fmt.Errorf("resource not found in state")
					}
					modelUUID := rs.Primary.Attributes["model_uuid"]
					if modelUUID == "" {
						return "", fmt.Errorf("model_uuid is empty in state")
					}
					return fmt.Sprintf("%s:alpha", modelUUID), nil
				},
				ExpectError: regexp.MustCompile("System Space Not Manageable"),
			},
		},
	})
}

func TestAcc_ResourceSpace_AlphaRejected(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-model")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceSpace(modelName, "alpha"),
				ExpectError: regexp.MustCompile("alpha is a system space and cannot be managed by juju_space"),
			},
		},
	})
}

func testAccResourceSpace(modelName, spaceName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_space" "this" {
  model_uuid = juju_model.this.uuid
  name       = %q
}
`, modelName, spaceName)
}

func testAccResourceSpaceModelOnly(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}
`, modelName)
}

func testAccCheckSpaceAbsent(spaceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if TestClient == nil {
			return fmt.Errorf("TestClient is not configured")
		}

		rs, ok := s.RootModule().Resources["juju_model.this"]
		if !ok {
			return fmt.Errorf("model resource not found in state")
		}

		modelUUID := rs.Primary.Attributes["model_uuid"]
		if modelUUID == "" {
			modelUUID = rs.Primary.Attributes["uuid"]
		}
		if modelUUID == "" {
			return fmt.Errorf("model uuid is empty in state")
		}

		_, err := TestClient.Spaces.ReadSpace(context.Background(), &juju.ReadSpaceInput{
			ModelUUID: modelUUID,
			Name:      spaceName,
		})
		if err == nil {
			return fmt.Errorf("space %q still exists in model %q", spaceName, modelUUID)
		}
		if jujuerrors.Is(err, jujuerrors.NotFound) {
			return nil
		}

		return fmt.Errorf("error checking whether space %q was destroyed: %w", spaceName, err)
	}
}
