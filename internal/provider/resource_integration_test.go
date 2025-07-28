// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAcc_ResourceIntegration(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName, "base = \"ubuntu@22.04\"", "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_integration.this",
			},
			{
				Config: testAccResourceIntegration(modelName, "base = \"ubuntu@22.04\"", "base = \"ubuntu@22.04\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
				),
			},
		},
	})
}

func TestAcc_ResourceIntegrationWithViaCIDRs(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")
	via := "127.0.0.1/32,127.0.0.3/32"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, "base = \"ubuntu@22.04\"", dstModelName, "base = \"ubuntu@22.04\"", via),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.a", "model", srcModelName),
					resource.TestCheckResourceAttr("juju_integration.a", "id", fmt.Sprintf("%v:%v:%v", srcModelName, "a:source", "b:sink")),
					resource.TestCheckResourceAttr("juju_integration.a", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.a", "application.*", map[string]string{"name": "a", "endpoint": "source"}),
					resource.TestCheckResourceAttr("juju_integration.a", "via", via),
				),
			},
		},
	})
}

func TestAcc_ResourceIntegration_UpgradeProvider(t *testing.T) {
	t.Skip("This test currently fails due to the breaking change in the provider schema. " +
		"Remove the skip after the v1 release of the provider.")

	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderStableVersion,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceIntegration(modelName, "series = \"jammy\"", "series = \"jammy\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceIntegration(modelName, "series = \"jammy\"", "series = \"jammy\""),
				PlanOnly:                 true,
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	return nil
}

func testAccResourceIntegration(modelName, osOne, osTwo string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model_uuid = juju_model.this.uuid
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		%s
	}
}

resource "juju_application" "two" {
	model_uuid = juju_model.this.uuid
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		%s
	}
}

resource "juju_integration" "this" {
	model = juju_model.this.name

	application {
		name     = juju_application.one.name
		endpoint = "source"
	}

	application {
		name = juju_application.two.name
		endpoint = "sink"
	}
}
`, modelName, osOne, osTwo)
}

// testAccResourceIntegrationWithVia generates a plan where a
// postgresql:source relates to a pgbouncer:backend-source using
// and offer of pgbouncer.
func testAccResourceIntegrationWithVia(srcModelName, aOS, dstModelName, bOS, viaCIDRs string) string {
	return fmt.Sprintf(`
resource "juju_model" "a" {
	name = %q
}

resource "juju_application" "a" {
	model_uuid = juju_model.a.uuid
	name  = "a" 
	
	charm {
		name = "juju-qa-dummy-sink"
		%s
	}
}

resource "juju_model" "b" {
	name = %q
}

resource "juju_application" "b" {
	model_uuid = juju_model.b.uuid
	name  = "b"
	
	charm {
		name = "juju-qa-dummy-source"
		%s
	}
}

resource "juju_offer" "b" {
	model            = juju_model.b.name
	application_name = juju_application.b.name
	endpoints         = ["sink"]
}

resource "juju_integration" "a" {
	model = juju_model.a.name
	via = %q

	application {
		name = juju_application.a.name
		endpoint = "source"
	}
	
	application {
		offer_url = juju_offer.b.url
	}
}
`, srcModelName, aOS, dstModelName, bOS, viaCIDRs)
}

func TestAcc_ResourceIntegrationWithMultipleConsumers(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationMultipleConsumers(srcModelName, dstModelName),
				ConfigVariables: config.Variables{
					"enable-b1-consumer": config.BoolVariable(true),
					"enable-b2-consumer": config.BoolVariable(true),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.b1.0", "model", dstModelName),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:source", "b1:sink")),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "sink"}),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "model", dstModelName),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:source", "b2:sink")),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b2.0", "application.*", map[string]string{"name": "b2", "endpoint": "sink"}),
				),
			},
			{
				Config: testAccResourceIntegrationMultipleConsumers(srcModelName, dstModelName),
				ConfigVariables: config.Variables{
					"enable-b1-consumer": config.BoolVariable(true),
					"enable-b2-consumer": config.BoolVariable(false),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.b1.0", "model", dstModelName),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:source", "b1:sink")),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "sink"}),
				),
			},
			{
				Config: testAccResourceIntegrationMultipleConsumers(srcModelName, dstModelName),
				ConfigVariables: config.Variables{
					"enable-b1-consumer": config.BoolVariable(false),
					"enable-b2-consumer": config.BoolVariable(false),
				},
			},
		},
	})
}

// testAccResourceIntegrationWithMultipleConusmers generates a plan where a
// two juju-qa-dummy-source applications relates to source offer.
func testAccResourceIntegrationMultipleConsumers(srcModelName string, dstModelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "a" {
        name = %q
}

resource "juju_application" "a" {
        model_uuid = juju_model.a.uuid
        name  = "a"

        charm {
                name    = "juju-qa-dummy-sink"
        }
}

resource "juju_offer" "a" {
        model            = juju_model.a.name
        application_name = juju_application.a.name
        endpoints         = ["source"]
}

resource "juju_model" "b" {
        name = %q
}

resource "juju_application" "b1" {
        model_uuid = juju_model.b.uuid
        name  = "b1"

        charm {
                name   = "juju-qa-dummy-source"
        }
}

resource "juju_integration" "b1" {
	count = var.enable-b1-consumer ? 1 : 0
        model = juju_model.b.name

        application {
                name     = juju_application.b1.name
                endpoint = "sink"
        }

        application {
                offer_url = juju_offer.a.url
        }
}

resource "juju_application" "b2" {
        model_uuid = juju_model.b.uuid
        name  = "b2"

        charm {
                name   = "juju-qa-dummy-source"
        }
}

resource "juju_integration" "b2" {
	count = var.enable-b2-consumer ? 1 : 0
        model = juju_model.b.name

        application {
                name     = juju_application.b2.name
                endpoint = "sink"
        }

        application {
                offer_url = juju_offer.a.url
        }
}

variable "enable-b1-consumer" {
	description = "Enable integration for b1 with offer"
	default     = false
}

variable "enable-b2-consumer" {
        description = "Enable integration for b2 with offer"
        default     = false
}
`, srcModelName, dstModelName)
}
