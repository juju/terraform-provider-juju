// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider_test

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/provider"
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

func TestValidateCACertificatesPEM_Success(t *testing.T) {
	v := provider.ValidateCACertificatesPEM()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, []string{testCert})
	if diags.HasError() {
		t.Fatalf("failed to build set: %v", diags)
	}
	req := validator.SetRequest{ConfigValue: set}
	var resp validator.SetResponse
	v.ValidateSet(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("expected no errors, got: %v", resp.Diagnostics)
	}
}

func TestValidateCACertificatesPEM_InvalidPEM(t *testing.T) {
	v := provider.ValidateCACertificatesPEM()
	set, diags := types.SetValueFrom(context.Background(), types.StringType, []string{"invalid"})
	if diags.HasError() {
		t.Fatalf("failed to build set: %v", diags)
	}
	req := validator.SetRequest{ConfigValue: set}
	var resp validator.SetResponse
	v.ValidateSet(context.Background(), req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected validation error for invalid PEM, got none")
	}
}

func TestValidateCACertificatesPEM_InvalidCertInsidePEM(t *testing.T) {
	v := provider.ValidateCACertificatesPEM()
	// Build a PEM block with valid headers but invalid DER payload
	payload := base64.StdEncoding.EncodeToString([]byte("not a cert"))
	invalidPEM := "-----BEGIN CERTIFICATE-----\n" + payload + "\n-----END CERTIFICATE-----\n"

	set, diags := types.SetValueFrom(context.Background(), types.StringType, []string{invalidPEM})
	if diags.HasError() {
		t.Fatalf("failed to build set: %v", diags)
	}
	req := validator.SetRequest{ConfigValue: set}
	var resp validator.SetResponse
	v.ValidateSet(context.Background(), req, &resp)
	if !resp.Diagnostics.HasError() {
		t.Fatalf("expected validation error for invalid cert payload, got none")
	}
}
