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

func TestAcc_ResourceIntegration_Edge(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName),
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
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
				),
			},
		},
	})
}

func TestAcc_ResourceIntegrationWithViaCIDRs_Edge(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")
	via := "127.0.0.1/32,127.0.0.3/32"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, dstModelName, via),
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

func TestAcc_ResourceIntegration_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		CheckDestroy: testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName),
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
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
				),
			},
		},
	})
}

func testAccCheckIntegrationDestroy(s *terraform.State) error {
	return nil
}

func testAccResourceIntegration(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model = juju_model.this.name
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		series = "jammy"
	}
}

resource "juju_application" "two" {
	model = juju_model.this.name
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		series = "jammy"
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
`, modelName)
}

func TestAcc_ResourceIntegrationWithViaCIDRs_Stable(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")
	via := "127.0.0.1/32,127.0.0.3/32"

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ExternalProviders: map[string]resource.ExternalProvider{
			"juju": {
				VersionConstraint: TestProviderStableVersion,
				Source:            "juju/juju",
			},
		},
		CheckDestroy: testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, dstModelName, via),
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

// testAccResourceIntegrationWithVia generates a plan where a
// postgresql:source relates to a pgbouncer:backend-source using
// and offer of pgbouncer.
func testAccResourceIntegrationWithVia(srcModelName string, dstModelName string, viaCIDRs string) string {
	return fmt.Sprintf(`
resource "juju_model" "a" {
	name = %q
}

resource "juju_application" "a" {
	model = juju_model.a.name
	name  = "a" 
	
	charm {
		name = "juju-qa-dummy-sink"
		series = "jammy"
	}
}

resource "juju_model" "b" {
	name = %q
}

resource "juju_application" "b" {
	model = juju_model.b.name
	name  = "b"
	
	charm {
		name = "juju-qa-dummy-source"
		series = "jammy"
	}
}

resource "juju_offer" "b" {
	model            = juju_model.b.name
	application_name = juju_application.b.name
	endpoint         = "sink"
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
`, srcModelName, dstModelName, viaCIDRs)
}

func TestAcc_ResourceIntegrationWithMultipleConsumers_Edge(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-dst")

	resource.Test(t, resource.TestCase{
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
					resource.TestCheckResourceAttr("juju_integration.b1.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:db-admin", "b1:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "backend-db-admin"}),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "model", dstModelName),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:db-admin", "b2:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b2.0", "application.*", map[string]string{"name": "b2", "endpoint": "backend-db-admin"}),
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
					resource.TestCheckResourceAttr("juju_integration.b1.0", "id", fmt.Sprintf("%v:%v:%v", dstModelName, "a:db-admin", "b1:backend-db-admin")),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "backend-db-admin"}),
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
// two pgbouncer applications relates to postgresql:db-admin offer.
func testAccResourceIntegrationMultipleConsumers(srcModelName string, dstModelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "a" {
        name = %q
}

resource "juju_application" "a" {
        model = juju_model.a.name
        name  = "a"

        charm {
                name    = "postgresql"
                series  = "jammy"
        }
}

resource "juju_offer" "a" {
        model            = juju_model.a.name
        application_name = juju_application.a.name
        endpoint         = "db-admin"
}

resource "juju_model" "b" {
        name = %q
}

resource "juju_application" "b1" {
        model = juju_model.b.name
        name  = "b1"

        charm {
                name   = "pgbouncer"
                series = "focal"
        }
}

resource "juju_integration" "b1" {
	count = var.enable-b1-consumer ? 1 : 0
        model = juju_model.b.name

        application {
                name     = juju_application.b1.name
                endpoint = "backend-db-admin"
        }

        application {
                offer_url = juju_offer.a.url
        }
}

resource "juju_application" "b2" {
        model = juju_model.b.name
        name  = "b2"

        charm {
                name   = "pgbouncer"
                series = "focal"
        }
}

resource "juju_integration" "b2" {
	count = var.enable-b2-consumer ? 1 : 0
        model = juju_model.b.name

        application {
                name     = juju_application.b2.name
                endpoint = "backend-db-admin"
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
