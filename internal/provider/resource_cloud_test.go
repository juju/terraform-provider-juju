// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAcc_ResourceCloud_OpenStack_Minimal(t *testing.T) {
	SkipJAAS(t) // cloud create/update may differ in JAAS for non-k8s clouds

	cloudName := "tf-test-cloud-openstack"

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccResourceCloud_OpenStack_Minimal(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "name", cloudName),
					resource.TestCheckResourceAttr("juju_cloud."+cloudName, "type", "openstack"),
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
  regions = [
    {
      name = "region-one"
    }
  ]
}
`
}
