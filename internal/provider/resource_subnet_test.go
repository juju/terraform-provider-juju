// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceSubnet_CreateImportUpdateAndDelete(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	testAccPreCheck(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	spaceA := "tf-space-a"
	spaceB := "tf-space-b"
	resourceFullName := "juju_subnet.this"
	subnetVars := config.Variables{}
	extractCIDRFromConfigVariablesMap := func(subnetVars config.Variables) (string, error) {
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

	steps := []resource.TestStep{
		// These first step create the model and extract the CIDR from alpha
		// space so that we have a valid CIDR to use in the subsequent subnet
		// resource creation step.
		{
			Config: testAccResourceSubnetModelAndSpaceOnly(modelName, spaceA),
			Check: func(s *terraform.State) error {
				rs, ok := s.RootModule().Resources["juju_model.this"]
				if !ok {
					return fmt.Errorf("resource %q not found in state", "juju_model.this")
				}
				modelUUID := rs.Primary.Attributes["uuid"]
				if modelUUID == "" {
					return fmt.Errorf("uuid is empty in state")
				}

				var err error
				cidr, err := getAlphaIPV4Subnet(context.Background(), modelUUID)
				if err != nil {
					return err
				}
				subnetVars["cidr"] = config.StringVariable(cidr)
				return nil
			},
		},
		// Create a subnet in spaceA.
		{
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionCreate),
				},
			},
			Config:          testAccResourceSubnet(modelName, spaceA),
			ConfigVariables: subnetVars,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttrPair(resourceFullName, "model_uuid", "juju_model.this", "uuid"),
				resource.TestCheckResourceAttrWith(resourceFullName, "cidr", func(actual string) error {
					cidr, err := extractCIDRFromConfigVariablesMap(subnetVars)
					if err != nil {
						return err
					}
					if cidr == "" {
						return fmt.Errorf("discovered cidr is empty")
					}
					if actual != cidr {
						return fmt.Errorf("expected cidr %q, got %q", cidr, actual)
					}
					return nil
				}),
				resource.TestCheckResourceAttr(resourceFullName, "space", spaceA),
				testAccCheckSubnetInSpace(resourceFullName, spaceA),
			),
		},
		// Import the subnet resource to verify the import logic works and
		// that the ID parsing logic is correct.
		{
			ResourceName:    resourceFullName,
			ImportState:     true,
			ConfigVariables: subnetVars,
			ImportStateIdFunc: func(s *terraform.State) (string, error) {
				rs, ok := s.RootModule().Resources[resourceFullName]
				if !ok {
					return "", fmt.Errorf("resource not found in state")
				}
				modelUUID := rs.Primary.Attributes["model_uuid"]
				if modelUUID == "" {
					return "", fmt.Errorf("model_uuid is empty in state")
				}
				subnetCIDR := rs.Primary.Attributes["cidr"]
				if subnetCIDR == "" {
					return "", fmt.Errorf("cidr is empty in state")
				}
				return fmt.Sprintf("%s:%s", modelUUID, subnetCIDR), nil
			},
			ImportStateVerify: true,
		},
		// Check the subnet is indeed assigned correctly to spaceB when we update the
		// resource to reference spaceB.
		{
			ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(resourceFullName, plancheck.ResourceActionUpdate),
				},
			},
			Config:          testAccResourceSubnet(modelName, spaceB),
			ConfigVariables: subnetVars,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(resourceFullName, "space", spaceB),
				testAccCheckSubnetInSpace(resourceFullName, spaceB),
			),
		},
		// Verify it goes back into alpha when subnet resource is removed.
		// This is a little concoluted because we need to extract the CIDR from our variable map
		// again, and check it is back in alpha manually via a ReadSubnet.
		{
			Config: testAccResourceSubnetModelAndSpaceOnly(modelName, spaceB),
			Check: func(s *terraform.State) error {
				rs, ok := s.RootModule().Resources["juju_model.this"]
				if !ok {
					return fmt.Errorf("resource %q not found in state", "juju_model.this")
				}
				modelUUID := rs.Primary.Attributes["uuid"]
				if modelUUID == "" {
					return fmt.Errorf("uuid is empty in state")
				}
				cidr, err := extractCIDRFromConfigVariablesMap(subnetVars)
				if err != nil {
					return err
				}
				return testAccCheckSubnetInSpaceByModelUUID(modelUUID, cidr, alphaSpaceName)
			},
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps:                    steps,
	})
}

func TestAcc_ResourceSubnet_CreateRejectedWhenNotAlpha(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	testAccPreCheck(t)

	modelName := acctest.RandomWithPrefix("tf-test-model")
	spaceA := "tf-space-a"
	spaceB := "tf-space-b"
	subnetVars := config.Variables{}

	steps := []resource.TestStep{
		{
			Config: testAccResourceSubnetModelAndSpacesOnly(modelName, spaceA, spaceB),
			Check: func(s *terraform.State) error {
				rs, ok := s.RootModule().Resources["juju_model.this"]
				if !ok {
					return fmt.Errorf("resource %q not found in state", "juju_model.this")
				}
				modelUUID := rs.Primary.Attributes["uuid"]
				if modelUUID == "" {
					return fmt.Errorf("uuid is empty in state")
				}

				var err error
				cidr, err := getAlphaIPV4Subnet(context.Background(), modelUUID)
				if err != nil {
					return err
				}
				subnetVars["cidr"] = config.StringVariable(cidr)
				return nil
			},
		},
		{
			Config:          testAccResourceSubnetConflictingCreate(modelName, spaceA, spaceB),
			ConfigVariables: subnetVars,
			ExpectError:     regexp.MustCompile("Subnet Not In Alpha Space"),
		},
	}

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps:                    steps,
	})
}

func testAccResourceSubnetModelAndSpaceOnly(modelName, spaceName string) string {
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

func testAccResourceSubnetModelAndSpacesOnly(modelName, firstSpaceName, secondSpaceName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
  name = %q
}

resource "juju_space" "first" {
	model_uuid = juju_model.this.uuid
	name       = %q
}

resource "juju_space" "second" {
	model_uuid = juju_model.this.uuid
	name       = %q
}
`, modelName, firstSpaceName, secondSpaceName)
}

func testAccResourceSubnet(modelName, spaceName string) string {
	return fmt.Sprintf(`
variable "cidr" {
	type = string
}

resource "juju_model" "this" {
  name = %q
}

resource "juju_space" "this" {
  model_uuid = juju_model.this.uuid
  name       = %q
}

resource "juju_subnet" "this" {
  model_uuid = juju_model.this.uuid
	cidr       = var.cidr
  space      = juju_space.this.name
}
`, modelName, spaceName)
}

func testAccResourceSubnetConflictingCreate(modelName, firstSpaceName, secondSpaceName string) string {
	return fmt.Sprintf(`
variable "cidr" {
	type = string
}

resource "juju_model" "this" {
	name = %q
}

resource "juju_space" "first" {
	model_uuid = juju_model.this.uuid
	name       = %q
}

resource "juju_space" "second" {
	model_uuid = juju_model.this.uuid
	name       = %q
}

resource "juju_subnet" "first" {
	model_uuid = juju_model.this.uuid
	cidr       = var.cidr
	space      = juju_space.first.name
}

resource "juju_subnet" "second" {
	depends_on = [juju_subnet.first]
	model_uuid = juju_model.this.uuid
	cidr       = var.cidr
	space      = juju_space.second.name
}
`, modelName, firstSpaceName, secondSpaceName)
}

func testAccCheckSubnetInSpace(resourceName, expectedSpace string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource %q not found in state", resourceName)
		}
		modelUUID := rs.Primary.Attributes["model_uuid"]
		if modelUUID == "" {
			return fmt.Errorf("model_uuid is empty in state")
		}
		cidr := rs.Primary.Attributes["cidr"]
		if cidr == "" {
			return fmt.Errorf("cidr is empty in state")
		}

		return testAccCheckSubnetInSpaceByModelUUID(modelUUID, cidr, expectedSpace)
	}
}

func testAccCheckSubnetInSpaceByModelUUID(modelUUID, cidr, expectedSpace string) error {
	if TestClient == nil {
		return fmt.Errorf("TestClient is not configured")
	}

	subnet, err := TestClient.Subnets.ReadSubnet(context.Background(), &juju.ReadSubnetInput{
		ModelUUID: modelUUID,
		CIDR:      cidr,
	})
	if err != nil {
		return fmt.Errorf("error reading subnet %q in model %q: %w", cidr, modelUUID, err)
	}
	if subnet.SpaceName != expectedSpace {
		return fmt.Errorf("expected subnet %q to be in space %q, got %q", cidr, expectedSpace, subnet.SpaceName)
	}
	return nil
}

// getAlphaIPV4Subnet discovers an available IPv4 subnet in the alpha space
// for the specified model.
func getAlphaIPV4Subnet(ctx context.Context, modelUUID string) (string, error) {
	if TestClient == nil {
		return "", fmt.Errorf("TestClient is not configured")
	}

	subnets, err := TestClient.Subnets.ListSubnets(ctx, &juju.ListSubnetsInput{
		ModelUUID: modelUUID,
		SpaceName: alphaSpaceName,
	})
	if err != nil {
		return "", fmt.Errorf("error listing subnets for model %q: %w", modelUUID, err)
	}

	for _, subnet := range subnets {
		ip, _, err := net.ParseCIDR(subnet.CIDR)
		if err != nil {
			continue
		}
		if subnet.SpaceName == alphaSpaceName && ip.To4() != nil {
			return subnet.CIDR, nil
		}
	}

	return "", fmt.Errorf("no alpha ipv4 subnet discovered in model %q", modelUUID)
}
