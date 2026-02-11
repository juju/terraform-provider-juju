// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAcc_ResourceIntegration(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")
	idCheck := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "one:source", "two:sink"))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_integration.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.this", tfjsonpath.New("id"), knownvalue.StringRegexp(idCheck)),
				},
			},
			{
				ImportStateVerify: true,
				ImportState:       true,
				ResourceName:      "juju_integration.this",
			},
			{
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_integration.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.this", tfjsonpath.New("id"), knownvalue.StringRegexp(idCheck)),
				},
			},
		},
	})
}

func TestAcc_ResourceIntegrationWithNullConfig(t *testing.T) {
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
				// Setting both an offer URL and name should error.
				Config: testAccResourceIntegrationWithNullVars(modelName),
				ConfigVariables: config.Variables{
					"set-offer-url": config.StringVariable("Y"),
					"set-name":      config.StringVariable("Y"),
				},
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// Setting neither an offer URL or name should error.
				Config:          testAccResourceIntegrationWithNullVars(modelName),
				ConfigVariables: config.Variables{},
				ExpectError:     regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				// Setting only name should work, while the other variable is null.
				Config: testAccResourceIntegrationWithNullVars(modelName),
				ConfigVariables: config.Variables{
					"set-name": config.StringVariable("Y"),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_integration.this", "model_uuid"),
				),
			},
		},
	})
}

func TestAcc_ResourceIntegrationUpdateIntegration(t *testing.T) {
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
				Config: testUpdateIntegrationToAppWithSameInterface(modelName, false),
			},
			{
				Config: testUpdateIntegrationToAppWithSameInterface(modelName, true),
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
	idCheck := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "a:source", "b:sink"))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		CheckDestroy:             testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationWithVia(srcModelName, "base = \"ubuntu@22.04\"", dstModelName, "base = \"ubuntu@22.04\"", via),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.a", "uuid", "juju_integration.a", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.a", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.a", "application.*", map[string]string{"name": "a", "endpoint": "source"}),
					resource.TestCheckResourceAttr("juju_integration.a", "via", via),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.a", tfjsonpath.New("id"), knownvalue.StringRegexp(idCheck)),
				},
			},
		},
	})
}

func TestAcc_ResourceIntegration_UpgradeV0ToV1(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	modelName := acctest.RandomWithPrefix("tf-test-integration")
	idCheck := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "one:source", "two:sink"))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		CheckDestroy: testAccCheckIntegrationDestroy,
		Steps: []resource.TestStep{
			{
				ExternalProviders: map[string]resource.ExternalProvider{
					"juju": {
						VersionConstraint: TestProviderPreV1Version,
						Source:            "juju/juju",
					},
				},
				Config: testAccResourceIntegrationV0(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_integration.this", "model", modelName),
					resource.TestCheckResourceAttr("juju_integration.this", "id", fmt.Sprintf("%v:%v:%v", modelName, "one:source", "two:sink")),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceIntegration(modelName),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.this", tfjsonpath.New("id"), knownvalue.StringRegexp(idCheck)),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_integration.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
			},
		},
	})
}

func TestAcc_ResourceIntegration_UpgradeProvider(t *testing.T) {
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
				Config: testAccResourceIntegration(modelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.this", "uuid", "juju_integration.this", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.this", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.this", "application.*", map[string]string{"name": "one", "endpoint": "source"}),
				),
			},
			{
				ProtoV6ProviderFactories: frameworkProviderFactories,
				Config:                   testAccResourceIntegration(modelName),
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
	model_uuid = juju_model.this.uuid
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "two" {
	model_uuid = juju_model.this.uuid
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_integration" "this" {
	model_uuid = juju_model.this.uuid

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

func testAccResourceIntegrationWithNullVars(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model_uuid = juju_model.this.uuid
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "two" {
	model_uuid = juju_model.this.uuid
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

variable "set-offer-url" {
  type = string
  default = null
  description = "A placeholder variable."
}

variable "set-name"{
  type = string
  default = null
  description = "Another placeholder variable."
}

resource "juju_integration" "this" {
	model_uuid = juju_model.this.uuid

	application {
		name     = juju_application.one.name
		endpoint = "source"
	}

	application {
        offer_url = var.set-offer-url != null ? "some-url" : null
        name      = var.set-name != null ? juju_application.two.name : null
	}
}
`, modelName)
}

func testAccResourceIntegrationV0(modelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model = juju_model.this.name
	name  = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "two" {
	model = juju_model.this.name
	name  = "two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
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

func testUpdateIntegrationToAppWithSameInterface(modelName string, relateToNewApp bool) string {
	appToRelate := "two"
	if relateToNewApp {
		appToRelate = "three"
	}
	return fmt.Sprintf(`
resource "juju_model" "this" {
	name = %q
}

resource "juju_application" "one" {
	model_uuid = juju_model.this.uuid
	name       = "one" 
	
	charm {
		name = "juju-qa-dummy-sink"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "two" {
	model_uuid = juju_model.this.uuid
	name       = "two"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_application" "three" {
	model_uuid = juju_model.this.uuid
	name       = "three"

	charm {
		name = "juju-qa-dummy-source"
		base = "ubuntu@22.04"
	}
}

resource "juju_integration" "this" {
	model_uuid = juju_model.this.uuid

	application {
		name     = juju_application.one.name
		endpoint = "source"
	}

	application {
		name = juju_application.%s.name
		endpoint = "sink"
	}
}
`, modelName, appToRelate)
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
	model_uuid       = juju_model.b.uuid
	application_name = juju_application.b.name
	endpoints        = ["sink"]
}

resource "juju_integration" "a" {
	model_uuid = juju_model.a.uuid
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
	id1Check := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "a:source", "b1:sink"))
	id2Check := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "a:source", "b2:sink"))

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
					resource.TestCheckResourceAttrPair("juju_model.b", "uuid", "juju_integration.b1.0", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "sink"}),
					resource.TestCheckResourceAttrPair("juju_model.b", "uuid", "juju_integration.b2.0", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.b2.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b2.0", "application.*", map[string]string{"name": "b2", "endpoint": "sink"}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.b1[0]", tfjsonpath.New("id"), knownvalue.StringRegexp(id1Check)),
					statecheck.ExpectKnownValue("juju_integration.b2[0]", tfjsonpath.New("id"), knownvalue.StringRegexp(id2Check)),
				},
			},
			{
				Config: testAccResourceIntegrationMultipleConsumers(srcModelName, dstModelName),
				ConfigVariables: config.Variables{
					"enable-b1-consumer": config.BoolVariable(true),
					"enable-b2-consumer": config.BoolVariable(false),
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.b", "uuid", "juju_integration.b1.0", "model_uuid"),
					resource.TestCheckResourceAttr("juju_integration.b1.0", "application.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("juju_integration.b1.0", "application.*", map[string]string{"name": "b1", "endpoint": "sink"}),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.b1[0]", tfjsonpath.New("id"), knownvalue.StringRegexp(id1Check)),
				},
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

func TestAcc_ResourceIntegrationWithMultipleIntegrationsSameEndpoint(t *testing.T) {
	if testingCloud != LXDCloudTesting {
		t.Skip(t.Name() + " only runs with LXD")
	}
	srcModelName := acctest.RandomWithPrefix("tf-test-integration-offering")
	dstModelName := acctest.RandomWithPrefix("tf-test-integration-consuming")
	idOneCheck := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "apptwo:source", "appzero:sink"))
	idTwoCheck := regexp.MustCompile(fmt.Sprintf(".+:%v:%v", "apptwo:source", "appone:sink"))

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceIntegrationMultipleIntegrationsSameEndpoint(srcModelName, dstModelName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("juju_model.consuming", "uuid", "juju_integration.this", "model_uuid"),
					resource.TestCheckResourceAttrPair("juju_model.consuming", "uuid", "juju_integration.this2", "model_uuid"),
				),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("juju_integration.this", tfjsonpath.New("id"), knownvalue.StringRegexp(idOneCheck)),
					statecheck.ExpectKnownValue("juju_integration.this2", tfjsonpath.New("id"), knownvalue.StringRegexp(idTwoCheck)),
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
        model_uuid       = juju_model.a.uuid
        application_name = juju_application.a.name
        endpoints        = ["source"]
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
        model_uuid = juju_model.b.uuid

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
        model_uuid = juju_model.b.uuid

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

func testAccResourceIntegrationMultipleIntegrationsSameEndpoint(srcModelName string, dstModelName string) string {
	return fmt.Sprintf(`
resource "juju_model" "offering" {
  name = %q
}

resource "juju_application" "appzero" {
  name  = "appzero"
  model_uuid = juju_model.offering.uuid

  charm {
    name = "juju-qa-dummy-source"
  }
  config = {
  	token = "abc"
  }
}

resource "juju_application" "appone" {
  name  = "appone"
  model_uuid = juju_model.offering.uuid

  charm {
    name = "juju-qa-dummy-source"
  }
  config = {
  	token = "abc"
  }
}

resource "juju_offer" "appzero_endpoint" {
  model_uuid       = juju_model.offering.uuid
  application_name = juju_application.appzero.name
  endpoints        = ["sink"]
}

resource "juju_offer" "appone_endpoint" {
  model_uuid       = juju_model.offering.uuid
  application_name = juju_application.appone.name
  endpoints        = ["sink"]
}

resource "juju_model" "consuming" {
  name = %q
}

resource "juju_application" "apptwo" {
  name       = "apptwo"
  model_uuid = juju_model.consuming.uuid

  charm {
    name = "juju-qa-dummy-sink"
  }
  config = {
  	token = "abc"
  }
}

resource "juju_integration" "this" {
  model_uuid = juju_model.consuming.uuid

  application {
    name     = juju_application.apptwo.name
    endpoint = "source"
  }

  application {
    offer_url = juju_offer.appzero_endpoint.url
  }
}

resource "juju_integration" "this2" {
  model_uuid = juju_model.consuming.uuid

  application {
    name     = juju_application.apptwo.name
    endpoint = "source"
  }

  application {
    offer_url = juju_offer.appone_endpoint.url
  }
}
`, srcModelName, dstModelName)
}
