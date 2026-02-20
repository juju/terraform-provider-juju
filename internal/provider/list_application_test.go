// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
)

func TestAccListApplications_query(t *testing.T) {
	modelName := acctest.RandomWithPrefix("tf-test-apps")
	appName := "my-application"

	var expectedID string

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccListApplicationsResourceConfig(modelName, appName),
				Check: func(s *terraform.State) error {
					appRes, ok := s.RootModule().Resources["juju_application.test"]
					if !ok {
						return fmt.Errorf("not found: juju_application.test")
					}
					expectedID = appRes.Primary.ID
					modelUUID, _, found := strings.Cut(expectedID, ":")
					if !found || modelUUID == "" {
						return fmt.Errorf("invalid application id format %q", expectedID)
					}
					return nil
				},
			},
			{
				Config: testAccListApplications(),
				Query:  true,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectIdentity("juju_application.test", map[string]knownvalue.Check{
						"id": knownvalue.StringFunc(func(actual string) error {
							return knownvalue.StringExact(expectedID).CheckValue(actual)
						}),
					}),
					querycheck.ExpectResourceKnownValues(
						"juju_application.test",
						queryfilter.ByResourceIdentity(map[string]knownvalue.Check{
							"id": knownvalue.StringFunc(func(actual string) error {
								return knownvalue.StringExact(expectedID).CheckValue(actual)
							}),
						}),
						[]querycheck.KnownValueCheck{
							{
								Path:       tfjsonpath.New("name"),
								KnownValue: knownvalue.StringExact(appName),
							},
							{
								Path: tfjsonpath.New("model_uuid"),
								KnownValue: knownvalue.StringFunc(func(actual string) error {
									expectedModelUUID, _, found := strings.Cut(expectedID, ":")
									if !found || expectedModelUUID == "" {
										return fmt.Errorf("invalid expected application id format %q", expectedID)
									}
									return knownvalue.StringExact(expectedModelUUID).CheckValue(actual)
								}),
							},
							{
								Path:       tfjsonpath.New("constraints"),
								KnownValue: knownvalue.StringExact("arch=arm64"),
							},
							{
								Path:       tfjsonpath.New("trust"),
								KnownValue: knownvalue.Bool(false),
							},
							{
								Path:       tfjsonpath.New("units"),
								KnownValue: knownvalue.NumberExact(big.NewFloat(1)),
							},
							{
								Path:       tfjsonpath.New("config").AtMapKey("hostname"),
								KnownValue: knownvalue.StringExact("diglett"),
							},
							// TODO: Can't deploy with storage rn for some reason
							// {
							// 	Path:       tfjsonpath.New("storage_directives").AtMapKey("files"),
							// 	KnownValue: knownvalue.StringExact("lxd,1,100M"),
							// },
							{
								Path:       tfjsonpath.New("resources"),
								KnownValue: knownvalue.Null(),
							},
							{
								Path:       tfjsonpath.New("endpoint_bindings"),
								KnownValue: knownvalue.Null(),
							},
							{
								Path:       tfjsonpath.New("registry_credentials"),
								KnownValue: knownvalue.Null(),
							},
							{
								Path:       tfjsonpath.New("charm").AtSliceIndex(0).AtMapKey("name"),
								KnownValue: knownvalue.StringExact("ubuntu"),
							},
							{
								Path:       tfjsonpath.New("charm").AtSliceIndex(0).AtMapKey("channel"),
								KnownValue: knownvalue.StringExact("latest/stable"),
							},
							{
								Path:       tfjsonpath.New("charm").AtSliceIndex(0).AtMapKey("revision"),
								KnownValue: knownvalue.NumberExact(big.NewFloat(24)),
							},
						},
					),
				},
			},
		},
	})
}

func testAccListApplicationsResourceConfig(modelName, appName string) string {
	return fmt.Sprintf(`
resource "juju_model" "test" {
	name        = %q
	constraints = "arch=arm64"
}

resource "juju_application" "test" {
	name       = %q
	model_uuid = juju_model.test.uuid

	charm {
		name     = "ubuntu"
		channel  = "latest/stable"
		revision = 24
	}

	config = {
		hostname = "diglett"
	}

	units = 1
}
`, modelName, appName)
}

func testAccListApplications() string {
	return `
list "juju_application" "test" {
  provider         = juju
	include_resource = true

  config {
    model_uuid = juju_model.test.uuid
  }
}
`
}
