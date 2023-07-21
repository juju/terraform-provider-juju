package provider

import (
	"context"
	"os"
	"runtime"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5to6server"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/assert"

	"github.com/rs/zerolog/log"
)

// providerFactories are used to instantiate a provider during acceptance testing.
// The factory function will be invoked for every Terraform CLI command executed
// to create a provider server to which the CLI can reattach.
var providerFactories map[string]func() (*schema.Provider, error)

// muxProviderFactories are used to instantiate the SDK provider and Framework provider
// during acceptance testing.
var muxProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// frameworkProviderFactories are used to instantiate the Framework provider during
// acceptance testing.
var frameworkProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// Provider makes a separate provider available for tests.
// Note that testAccPreCheck needs to invoked before use.
var Provider *schema.Provider

func init() {
	Provider = New("dev")()

	upgradedSdkProvider, err := tf5to6server.UpgradeServer(
		context.Background(),
		Provider.GRPCProvider,
	)
	if err != nil {
		log.Fatal().Msg(err.Error())
	}

	muxProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev")),
		"oldjuju": func() (tfprotov6.ProviderServer, error) {
			return upgradedSdkProvider, err
		},
	}
	frameworkProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev")),
	}
}

func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProviderConfigure(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := New("dev")()
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if diags != nil {
		t.Errorf("%+v", diags)
	}
}

func TestProviderConfigureUsernameFromEnv(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := New("dev")()
	userNameValue := "the-username"
	t.Setenv(JujuUsernameEnvKey, userNameValue)
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	assert.Len(t, diags, 1)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, diag.Error, diags[0].Severity)
	assert.Equal(t, "invalid entity name or password (unauthorized access)", diags[0].Detail)
}

func TestProviderConfigurePasswordFromEnv(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := New("dev")()
	passwordValue := "the-password"
	t.Setenv(JujuPasswordEnvKey, passwordValue)
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	assert.Len(t, diags, 1)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, diag.Error, diags[0].Severity)
	assert.Equal(t, "invalid entity name or password (unauthorized access)", diags[0].Detail)
}

func TestProviderConfigureAddresses(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := New("dev")()
	// This IP is from a test network that should never be routed. https://www.rfc-editor.org/rfc/rfc5737#section-3
	t.Setenv(JujuControllerEnvKey, "192.0.2.100:17070")
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	assert.Len(t, diags, 1)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, diag.Error, diags[0].Severity)
	assert.Equal(t, "dial tcp 192.0.2.100:17070: i/o timeout", diags[0].Summary)
	assert.Equal(t, "Connection error, please check the controller_addresses property set on the provider", diags[0].Detail)
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
	jujuProvider := New("dev")()
	t.Setenv(JujuCACertEnvKey, invalidCA)
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	assert.Len(t, diags, 1)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, diag.Error, diags[0].Severity)
	assert.Equal(t, "Verify the ca_certificate property set on the provider", diags[0].Detail)
	assert.Equal(t, "x509: certificate signed by unknown authority", diags[0].Summary)
}

func TestProviderConfigurex509InvalidFromEnv(t *testing.T) {
	jujuProvider := New("dev")()
	//Set the CA to the invalid one above
	//Juju will ignore the system trust store if we set the CA property
	t.Setenv(JujuCACertEnvKey, invalidCA)
	t.Setenv("JUJU_CA_CERT_FILE", "")
	diags := jujuProvider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	assert.Len(t, diags, 1)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, diag.Error, diags[0].Severity)
	assert.Equal(t, "Verify the ca_certificate property set on the provider", diags[0].Detail)
	assert.Equal(t, "x509: certificate signed by unknown authority", diags[0].Summary)
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

	err := Provider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if err != nil {
		t.Fatal(err)
	}
}

func TestFrameworkProviderSchema(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev")
	req := frameworkprovider.SchemaRequest{}
	resp := frameworkprovider.SchemaResponse{}
	jujuProvider.Schema(context.Background(), req, &resp)
	assert.Equal(t, resp.Diagnostics.HasError(), false)
	assert.Len(t, resp.Schema.Attributes, 4)
}
