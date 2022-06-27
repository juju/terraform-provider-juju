package provider

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// providerFactories are used to instantiate a provider during acceptance testing.
// The factory function will be invoked for every Terraform CLI command executed
// to create a provider server to which the CLI can reattach.
var providerFactories = map[string]func() (*schema.Provider, error){
	"juju": func() (*schema.Provider, error) {
		return New("dev")(), nil
	},
}

func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
	}
}

func TestProviderConfigure(t *testing.T) {
	testAccPreCheck(t)
	provider := New("dev")()
	diags := provider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if diags != nil {
		t.Errorf("%+v", diags)
	}
}

func TestProviderConfigureUsername(t *testing.T) {
	testAccPreCheck(t)
	provider := New("dev")()
	t.Setenv(JujuUsernameEnvKey, "")
	diags := provider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if diags == nil {
		t.Errorf("provider should error")
	}
	err := diags[len(diags)-1]
	if err.Summary != "Username and password must be set" && err.Detail != "Currently the provider can only authenticate using username and password based authentication, if both are empty the provider will panic" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestProviderConfigurePassword(t *testing.T) {
	testAccPreCheck(t)
	provider := New("dev")()
	t.Setenv(JujuPasswordEnvKey, "")
	diags := provider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if diags == nil {
		t.Errorf("provider should error")
	}
	err := diags[len(diags)-1]
	if err.Summary != "Username and password must be set" && err.Detail != "Currently the provider can only authenticate using username and password based authentication, if both are empty the provider will panic" {
		t.Errorf("unexpected error: %+v", err)
	}
}

func TestProviderConfigureAddresses(t *testing.T) {
	testAccPreCheck(t)
	provider := New("dev")()
	// This IP is from a test network that should never be routed. https://www.rfc-editor.org/rfc/rfc5737#section-3
	t.Setenv(JujuControllerEnvKey, "192.0.2.100:17070")
	diags := provider.Configure(context.Background(), terraform.NewResourceConfigRaw(nil))
	if diags == nil {
		t.Errorf("provider should error")
	}
	err := diags[len(diags)-1]
	if !strings.Contains(err.Summary, "dial tcp 192.0.2.100:17070:") {
		t.Errorf("unexpected error: %+v", err)
	}
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
}
