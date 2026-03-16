// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	internaltesting "github.com/juju/terraform-provider-juju/internal/testing"
)

// charmProviderFactories spins up the provider without a Juju controller
// connection. The juju_charm data source only queries CharmHub directly.
var charmProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev", ProviderConfiguration{})),
}

func TestAcc_DataSourceCharm_Basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: charmProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceCharm_basic("juju-jimm-k8s", "3/stable", "ubuntu@22.04", 0),
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("data.juju_charm.test", "revision",
						regexp.MustCompile(`^[1-9][0-9]*$`)),
					resource.TestCheckResourceAttr("data.juju_charm.test", "channel", "3/stable"),
					resource.TestCheckResourceAttr("data.juju_charm.test", "requires.ingress", "ingress"),
				),
			},
		},
	})
}

func testAccDataSourceCharm_basic(name string, channel string, base string, revision int) string {
	return internaltesting.GetStringFromTemplateWithData(
		"testAccDataSourceCharm_basic",
		`
data "juju_charm" "test" {
  charm   = "{{ .Name }}"
  base    = "{{ .Base }}"
  {{- if ne .Revision 0 }}
  	revision = {{ .Revision }}
  {{- end }}
  {{- if ne .Channel "" }}
	channel = "{{ .Channel }}"
  {{- end }}
}
`, internaltesting.TemplateData{
			"Name":     name,
			"Channel":  channel,
			"Base":     base,
			"Revision": revision,
		})
}

func TestAcc_DataSourceCharm_RelationInterface(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: charmProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDataSourceCharm_basic("juju-jimm-k8s", "3/edge", "ubuntu@22.04", 104),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.juju_charm.test", "revision", "104"),
					resource.TestCheckResourceAttr("data.juju_charm.test", "channel", "3/edge"),
					resource.TestCheckResourceAttr("data.juju_charm.test", "requires.internal-ingress", "ingress"),
					resource.TestCheckResourceAttr("data.juju_charm.test", "requires.ingress", "ingress"),
					resource.TestCheckResourceAttr("data.juju_charm.test", "resources.jimm-image", "60"),
				),
			},
		},
	})
}

func TestUnit_DataSourceCharm_InvalidBase(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: charmProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      testAccDataSourceCharm_basic("juju-jimm-k8s", "3/edge", "ubuntu-22.04", 0),
				ExpectError: regexp.MustCompile(`must be in the form os@channel`),
			},
			{
				Config:      testAccDataSourceCharm_basic("juju-jimm-k8s", "3/edge", "ubuntu@", 0),
				ExpectError: regexp.MustCompile(`must be in the form os@channel`),
			},
			{
				Config:      testAccDataSourceCharm_basic("juju-jimm-k8s", "3/edge", "@22.04", 0),
				ExpectError: regexp.MustCompile(`must be in the form os@channel`),
			},
		},
	})
}
