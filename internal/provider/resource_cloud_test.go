// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

var (
	testCert = `-----BEGIN CERTIFICATE-----
MIIFlTCCA32gAwIBAgIUKRpWKB9RYEomRPcKuUkpVoBpix4wDQYJKoZIhvcNAQEL
BQAwWTESMBAGA1UEAwwJbG9jYWxob3N0MQswCQYDVQQGEwJVSzEQMA4GA1UECAwH
RGlnbGV0dDEQMA4GA1UEBwwHRGlnbGV0dDESMBAGA1UECgwJQ2Fub25pY2FsMCAX
DTI1MDgxMzExMTYzN1oYDzIxMjUwNzIwMTExNjM3WjBZMRIwEAYDVQQDDAlsb2Nh
bGhvc3QxCzAJBgNVBAYTAlVLMRAwDgYDVQQIDAdEaWdsZXR0MRAwDgYDVQQHDAdE
aWdsZXR0MRIwEAYDVQQKDAlDYW5vbmljYWwwggIiMA0GCSqGSIb3DQEBAQUAA4IC
DwAwggIKAoICAQDIaKN+7hah2lZUi8WULF7yv+fweE8HpeyAz/tkQeqe4Gpo9L1/
BssRWA12iRAS+Tfp6FKptwBaxwvmOML5NvqelwgKYqM6UwWfkjFGsWbXFh2ME1/F
k7LYghektEbo5835NJHEDj/fta5BEOsmoA9U2zB8lx7la9qqoXMiB+Ght+2faOI9
W7bd2tLfeesll5tBBWrN8MEmeF38jqS9zuNYnnc4uq+fAmg31OmwrrGuXA96lfS7
tGBcSd9oW0xc+9j2ufvvdzjJRaZWSGkG9K/QnTdNL+UAGwh6ihz188VCnlW+eHoS
tU0iRttNqJHdjOyM0E1Ux2/rda7Ouarfr49NkjZDSjT5Da3TqmRqAHEYOTzDHaof
Wm0YKsqXHcuWCRzA/K4zAFpPimYhojY4gTv7XGjdInfoI5ERJWzKuf+0wGmrFRZD
Di/ygPlfz6CBZ+YQvSbSlUKb61cU7hqP0mOpR4Sh7zFypk7O08n7/qa823L2UeS+
HhLaz1y+uaHIBDD9BQGLCFdI9qqokbt7Nv6cYjeGQxbLJ1ChGoxA6hUwH/TYnT1k
QYYIlyJ4VeDD3J7Oo6y0NTEgxyzLnhHkEheP7xq9/PZ2FwqTwDZ0bHWOxLosMbZM
spzzGLUjmLqRWEXU7BMKE2dEwZwqbTlYl63gS/DX13abo47OkxMSmbsV0QIDAQAB
o1MwUTAdBgNVHQ4EFgQUVrHhDhcXzM0zgLmXej7tdN5FzagwHwYDVR0jBBgwFoAU
VrHhDhcXzM0zgLmXej7tdN5FzagwDwYDVR0TAQH/BAUwAwEB/zANBgkqhkiG9w0B
AQsFAAOCAgEAdvKOxgD0z8OZL7ng4Q4yIbGXyzDITryMZ+t2ApWitWuYUvYt2hgB
15fQU8RHojyrcBk9HI4SLwcH285UWGW3kdMk47S83qfBjVuo7Ct6HG27ee1U1u9d
jGZckldwnKdEvo5Tg3UUfEWgWMjYIkjNIySSrIhJjVrHMGAuOEVGN6lDPUkEh2h5
/VJiOjSdso7XohTjCfa8TrnBnb1EVfoIPeby6UDky9mbwbCn0jpI09PHmWwvv5PH
1DQO4/ouB1ZeYrkS5nQgFpFYOPa4ctyHqhIV3lgfwZ1IWSt+bFqHWek5eKhp8dYb
I3rvgiGPR9+aNadEuc+kpXl+sdv+k1j25Q9eIMpwcU1xjdmZtaM85SGeucPHfMMY
5F2VVXRhOvMscaj/08n1xvRtAhhx0ymfg9BW2HkObA7V+44ns8v4f4/R5y+lZVZx
oZFRj/7pcDoNcQTq/fdS8NmElyqbqHjcgB0WQsg0heWLeKSWZvZEUvE3p+wmi0q1
F9g0aK3+1wKSKp9FMJzGTgRiOdVNEVzAulbnfhQh3LYLBYCGvNZ1RWz5nJvCp5Yc
2gFXkwP5bMT6bG8/9VC5+VQwTzOkljZaYW7KPeDVyKx7u38AFT/D9Q0qsDt+czL4
TiQuXo1Umr0AZLv9jFBEkvVJAdgURVouG/lnidQ3apg4NcAWv2cpOBQ=
-----END CERTIFICATE-----
`
)

func TestAcc_ResourceCloud(t *testing.T) {
	SkipJAAS(t)

	cloudName := acctest.RandomWithPrefix("tf-test-cloud")
	resourceName := "juju_cloud." + cloudName

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{
			// Only required fields.
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
			// Update in-place every other field (they're all update in place).
			{
				Config: testAccResourceCloud_OpenStack_AllFields(cloudName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", cloudName),
					resource.TestCheckResourceAttr(resourceName, "type", "openstack"),
					resource.TestCheckResourceAttr(resourceName, "auth_types.#", "2"),
					resource.TestCheckTypeSetElemAttr(resourceName, "auth_types.*", "userpass"),
					resource.TestCheckTypeSetElemAttr(resourceName, "auth_types.*", "access-key"),
					resource.TestCheckResourceAttr(resourceName, "endpoint", "https://cloud.example.com"),
					resource.TestCheckResourceAttr(resourceName, "identity_endpoint", "https://identity.example.com"),
					resource.TestCheckResourceAttr(resourceName, "storage_endpoint", "https://storage.example.com"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.0", testCert+"\n"),
					resource.TestCheckResourceAttr(resourceName, "ca_certificates.1", testCert+"\n"),
					resource.TestCheckResourceAttr(resourceName, "regions.#", "2"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.name", "default"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.endpoint", "https://region-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.identity_endpoint", "https://identity-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.0.storage_endpoint", "https://storage-default.example.com"),
					resource.TestCheckResourceAttr(resourceName, "regions.1.name", "us-east-1"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.1.endpoint"),
					resource.TestCheckNoResourceAttr(resourceName, "regions.1.identity_endpoint"),
					resource.TestCheckResourceAttr(resourceName, "regions.1.storage_endpoint", "https://storage-us-east-1.example.com"),
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

func testAccResourceCloud_OpenStack_AllFields(name string) string {
	return `
resource "juju_cloud" "` + name + `" {
  name = "` + name + `"
  type = "openstack"

  # multiple auth types
  auth_types = ["userpass", "access-key"]

  # global endpoints
  endpoint = "https://cloud.example.com"
  identity_endpoint = "https://identity.example.com"
  storage_endpoint  = "https://storage.example.com"

  # two identical CA certificates for testing
  ca_certificates = [
<<-CERT
` + testCert + `
CERT
,
<<-CERT
` + testCert + `
CERT
  ]

  # multiple regions with per-region endpoints
  regions = [
    {
      name = "default"
      endpoint = "https://region-default.example.com"
      identity_endpoint = "https://identity-default.example.com"
      storage_endpoint  = "https://storage-default.example.com"
    },
    {
      name = "us-east-1"
      # leave endpoint and identity_endpoint unset to verify nullability
      storage_endpoint  = "https://storage-us-east-1.example.com"
    }
  ]
}
`
}
