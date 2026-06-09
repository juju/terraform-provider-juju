// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcc_DataSourceSubnets(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}

	modelName := "tf-datasource-subnets-test-model"
	subnetVars := config.Variables{}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceSubnetsModelOnly(modelName),
				Check: func(s *terraform.State) error {
					rs, ok := s.RootModule().Resources["juju_model.this"]
					if !ok {
						return fmt.Errorf("resource %q not found in state", "juju_model.this")
					}

					modelUUID := rs.Primary.Attributes["uuid"]
					if modelUUID == "" {
						return fmt.Errorf("uuid is empty in state")
					}

					cidr, err := getAlphaIPV4Subnet(context.Background(), modelUUID)
					if err != nil {
						return err
					}
					subnetVars["cidr"] = config.StringVariable(cidr)
					return nil
				},
			},
			{
				Config:          testAccDataSourceSubnets(modelName),
				ConfigVariables: subnetVars,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.juju_subnets.this", "model_uuid", "juju_model.this", "uuid"),
					testCheckMapNotEmpty("data.juju_subnets.this", "subnets"),
					testAccCheckDataSourceSubnetsEntry("data.juju_subnets.this", subnetVars),
				),
			},
		},
	})
}

func extractCIDRFromConfigVariablesMap(subnetVars config.Variables) (string, error) {
	cidrJSON, err := subnetVars["cidr"].MarshalJSON()
	if err != nil {
		return "", fmt.Errorf("failed to marshal cidr: %w", err)
	}

	var cidr string
	if err := json.Unmarshal(cidrJSON, &cidr); err != nil {
		return "", fmt.Errorf("failed to unmarshal cidr: %w", err)
	}

	return cidr, nil
}

func testAccCheckDataSourceSubnetsEntry(resourceName string, subnetVars config.Variables) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found", resourceName)
		}

		cidr, err := extractCIDRFromConfigVariablesMap(subnetVars)
		if err != nil {
			return err
		}
		if cidr == "" {
			return fmt.Errorf("discovered cidr is empty")
		}

		requiredAttrs := []string{
			fmt.Sprintf("subnets.%s.cidr", cidr),
			fmt.Sprintf("subnets.%s.space_name", cidr),
			fmt.Sprintf("subnets.%s.provider_id", cidr),
			fmt.Sprintf("subnets.%s.provider_network_id", cidr),
			fmt.Sprintf("subnets.%s.provider_space_id", cidr),
			fmt.Sprintf("subnets.%s.vlan_tag", cidr),
			fmt.Sprintf("subnets.%s.life", cidr),
			fmt.Sprintf("subnets.%s.zones.#", cidr),
		}

		for _, attr := range requiredAttrs {
			if _, present := rs.Primary.Attributes[attr]; !present {
				return fmt.Errorf("attribute %q not found", attr)
			}
		}

		if got := rs.Primary.Attributes[fmt.Sprintf("subnets.%s.cidr", cidr)]; got != cidr {
			return fmt.Errorf("expected cidr %q, got %q", cidr, got)
		}

		if got := rs.Primary.Attributes[fmt.Sprintf("subnets.%s.space_name", cidr)]; got != alphaSpaceName {
			return fmt.Errorf("expected space_name %q, got %q", alphaSpaceName, got)
		}

		return nil
	}
}

func testCheckMapNotEmpty(resourceName, attr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found", resourceName)
		}

		count, ok := rs.Primary.Attributes[attr+".%"]
		if !ok || count == "0" {
			return fmt.Errorf("%s is empty", attr)
		}

		return nil
	}
}

func testAccDataSourceSubnetsModelOnly(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}
`, modelName)
}

func testAccDataSourceSubnets(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

data "juju_subnets" "this" {
  model_uuid = juju_model.this.uuid
}
`, modelName)
}
