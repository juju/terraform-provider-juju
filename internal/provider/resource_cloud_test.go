// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceCloud_RequiredOnly(t *testing.T) {
	SkipJAAS(t) // cloud create/update may differ in JAAS for non-k8s clouds

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")
	resourceName := "juju_cloud." + cloudName

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_Minimal(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", cloudName),
					resource.TestCheckResourceAttr(resourceName, "type", "openstack"),
					resource.TestCheckResourceAttr(resourceName, "regions.#", "1"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.name", "default"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.identity_endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.0.storage_endpoint"),
				),
			},
		},
	})
}

func TestAcc_ResourceCloud_WithRegion(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_WithRegion(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "name", cloudName),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "type", "openstack"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.#", "1"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.0.name", "region-one"),
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "regions.0.endpoint"),
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "regions.0.identity_endpoint"),
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "regions.0.storage_endpoint"),
				),
			},
		},
	})
}

func TestAcc_ResourceCloud_MultipleRegions(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_MultipleRegions(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "name", cloudName),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "type", "openstack"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.#", "2"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.0.name", "region-one"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.1.name", "region-two"),
				),
			},
		},
	})
}

func TestAcc_ResourceCloud_UpdateRegions(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_WithRegion(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.#", "1"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.0.name", "region-one"),
				),
			},
			{
				Config: testAccResourceCloud_OpenStack_MultipleRegions(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.#", "2"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.0.name", "region-one"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "regions.1.name", "region-two"),
				),
			},
		},
	})
}

func TestAcc_ResourceCloud_EmptyRegionsError(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccResourceCloud_OpenStack_EmptyRegions(cloudName),
				ExpectError: regexp.MustCompile("must contain at least one region"),
			},
		},
	})
}

func TestAcc_ResourceCloud_TopLevelEndpoints(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_Minimal(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "endpoint"),
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "identity_endpoint"),
					resource.TestCheckNoResourceAttr("juju_cloud."+cloudName, "storage_endpoint"),
				),
			},
			{
				Config: testAccResourceCloud_OpenStack_WithEndpoints(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "endpoint", "https://api.example.com"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "identity_endpoint", "https://id.example.com"),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "storage_endpoint", "https://obj.example.com"),
				),
			},
		},
	})
}

func testAccResourceCloud_OpenStack_Minimal(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
}
`
}

func testAccResourceCloud_OpenStack_WithRegion(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
  regions = [
    {
      name = "region-one"
    }
  ]
}
`
}

func testAccResourceCloud_OpenStack_MultipleRegions(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
  regions = [
    { name = "region-one" },
    { name = "region-two" }
  ]
}
`
}

func testAccResourceCloud_OpenStack_EmptyRegions(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
  regions = []
}
`
}

func testAccResourceCloud_OpenStack_WithEndpoints(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"
  auth_types = ["userpass"]
  endpoint = "https://api.example.com"
  identity_endpoint = "https://id.example.com"
  storage_endpoint = "https://obj.example.com"
}
`
}
