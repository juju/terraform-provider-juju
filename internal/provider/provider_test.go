// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"os"
	"runtime"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

const TestProviderStableVersion = "0.9.1"

// providerFactories are used to instantiate the Framework provider during
// acceptance testing.
var frameworkProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// Provider makes a separate provider available for tests.
// Note that testAccPreCheck needs to invoked before use.
var Provider provider.Provider

// TestClient is needed for any resource to be able to use Juju client in
// custom checkers for their tests (e.g. resource_model_test)
var TestClient *juju.Client

func init() {
	Provider = NewJujuProvider("dev")

	frameworkProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev")),
	}
}

func TestProviderConfigure(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	confResp := configureProvider(t, jujuProvider)
	assert.Equal(t, confResp.Diagnostics.HasError(), false)
}

func TestProviderConfigureUsernameFromEnv(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	userNameValue := "the-username"
	t.Setenv(JujuUsernameEnvKey, userNameValue)

	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "invalid entity name or password (unauthorized access)", err.Detail())
}

func TestProviderConfigurePasswordFromEnv(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	passwordValue := "the-password"
	t.Setenv(JujuPasswordEnvKey, passwordValue)
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "invalid entity name or password (unauthorized access)", err.Detail())
}

func TestProviderConfigureAddresses(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	// This IP is from a test network that should never be routed. https://www.rfc-editor.org/rfc/rfc5737#section-3
	t.Setenv(JujuControllerEnvKey, "192.0.2.100:17070")
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "dial tcp 192.0.2.100:17070: i/o timeout", err.Summary())
	assert.Equal(t, "Connection error, please check the controller_addresses property set on the provider", err.Detail())
}

// This is a valid certificate allowing the client to attempt a connection but failing certificate validation
const (
	invalidCA = "-----BEGIN CERTIFICATE-----\nMIIDazCCAlOgAwIBAgIULHtYyq/mjGAaZTTFcfd4Dmi6LtkwDQYJKoZIhvcNAQEL\nBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM\nGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjA2MjQxNTQzMTFaFw0yMjA3\nMjQxNTQzMTFaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw\nHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQCgSrxunimy/Nig3y5mAUtc3quvJI7MVdlWrhhWcNP4\nacF6bsAYDMa02Praf3pUBkyU9Fe83nalcimVO1NO18/FvKK4ZYuwQi4B+Rx1ltF/\nZx5czxrH+kb9FsZJNAtxbAo0hT9rusuCd1m0zhzSOZCTWkmguDew41IQHUtW7Wgy\nM0TlmrCzJkf2w+GwmhxFbJLR37b7N2ylyrFyuLTEKSMAxSw7k4+Djqgat5NdVGmo\niTZST86Br9Xg+goVjFTHxj/f84OaazM6DhyIdizyntkIV6nZVxZmhisO9iWk41Q/\noPeN4ZYUCe+VpZoZShMZ7H281tOYfgCOP2IHyQxxwLQBAgMBAAGjUzBRMB0GA1Ud\nDgQWBBS1ziAYMPkbTHaOfgpKlX70/wkusDAfBgNVHSMEGDAWgBS1ziAYMPkbTHaO\nfgpKlX70/wkusDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAN\n76z4TTrH5Wj7nPBROyx9Ab3TCF+gSqi2lhxCo5obtdAUdnfsbTtIGH82Ayduz13R\nvWcqn0EXgi2jJ8fMQxujalBwqhw2BPLgXPhIlR8/IcvUp9CIQA3FasvqNrSrfUzJ\ntO9oA3LG5EGnlxeDS5ehkx/bAOQl4yz70Vh+xssU/E5T74Zb8Kgf8uSZbj2jbRh7\nBC4qYzO7jVFOLkIWUjIeKlE2iG3OJnb17NMuODApPLyRslKvRyxwITtWr/jhaTNQ\n4L64mCtPPU2bMLScqsEYDOx237na8m9Xej6MOGb1D4noe59ML/4IwCmG2iK982mQ\n2zpE+UCo97FGq4kDK6bc\n-----END CERTIFICATE-----\n"
)

// TODO: find an alternative way of running test on Mac
func TestProviderConfigurex509FromEnv(t *testing.T) {
	if runtime.GOOS == "darwin" {
		//Due to a bug in Go this test does not work on darwin OS
		//https://github.com/golang/go/issues/52010
		t.Skip("This test does not work on MacOS")
	}
	jujuProvider := NewJujuProvider("dev")
	t.Setenv(JujuCACertEnvKey, invalidCA)
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "Verify the ca_certificate property set on the provider", err.Detail())
	assert.Equal(t, "x509: certificate signed by unknown authority", err.Summary())
}

func TestProviderConfigurex509InvalidFromEnv(t *testing.T) {
	jujuProvider := NewJujuProvider("dev")
	//Set the CA to the invalid one above
	//Juju will ignore the system trust store if we set the CA property
	t.Setenv(JujuCACertEnvKey, invalidCA)
	t.Setenv("JUJU_CA_CERT_FILE", "")
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "Verify the ca_certificate property set on the provider", err.Detail())
	assert.Equal(t, "x509: certificate signed by unknown authority", err.Summary())
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv(JujuUsernameEnvKey); v == "" {
		t.Fatalf("%s must be set for acceptance tests", JujuUsernameEnvKey)
	}
	if v := os.Getenv(JujuPasswordEnvKey); v == "" {
		t.Fatalf("%s must be set for acceptance tests", JujuPasswordEnvKey)
	}
	if v := os.Getenv(JujuCACertEnvKey); v == "" {
		if v := os.Getenv("JUJU_CA_CERT_FILE"); v != "" {
			t.Logf("reading certificate from: %s", v)
			cert, err := os.ReadFile(v)
			if err != nil {
				t.Fatalf("cannot read file specified by JUJU_CA_CERT_FILE for acceptance tests: %s", err)
			}
			os.Setenv(JujuCACertEnvKey, string(cert))
		} else {
			t.Fatalf("%s must be set for acceptance tests", JujuCACertEnvKey)
		}
	}
	confResp := configureProvider(t, Provider)
	assert.Equal(t, confResp.Diagnostics.HasError(), false)
	TestClient = confResp.ResourceData.(*juju.Client)
}

func configureProvider(t *testing.T, p provider.Provider) provider.ConfigureResponse {
	schemaResp := provider.SchemaResponse{}
	Provider.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)
	assert.Equal(t, schemaResp.Diagnostics.HasError(), false)

	conf := jujuProviderModel{}

	mapTypes := map[string]attr.Type{
		JujuController: types.StringType,
		JujuUsername:   types.StringType,
		JujuPassword:   types.StringType,
		JujuCACert:     types.StringType,
	}

	val, confObjErr := types.ObjectValueFrom(context.Background(), mapTypes, conf)
	assert.Equal(t, confObjErr.HasError(), false)

	tfval, tfValErr := val.ToTerraformValue(context.Background())
	assert.Equal(t, tfValErr, nil)

	c := tfsdk.Config{Schema: schemaResp.Schema, Raw: tfval}
	confReq := provider.ConfigureRequest{Config: c}
	confResp := provider.ConfigureResponse{Diagnostics: diag.Diagnostics{}}

	p.Configure(context.Background(), confReq, &confResp)

	return confResp
}

func TestFrameworkProviderSchema(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	req := provider.SchemaRequest{}
	resp := provider.SchemaResponse{}
	jujuProvider.Schema(context.Background(), req, &resp)
	assert.Equal(t, resp.Diagnostics.HasError(), false)
	assert.Len(t, resp.Schema.Attributes, 4)
}
