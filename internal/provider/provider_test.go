package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// providerFactories are used to instantiate a provider during acceptance testing.
// The factory function will be invoked for every Terraform CLI command executed
// to create a provider server to which the CLI can reattach.
var providerFactories = map[string]func() (*schema.Provider, error){
	"juju": func() (*schema.Provider, error) {
		return New("dev")(), nil
	},
}

// TODO: Expand these tests
func TestProvider(t *testing.T) {
	if err := New("dev")().InternalValidate(); err != nil {
		t.Fatalf("err: %s", err)
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
		t.Fatalf("%s must be set for acceptance tests", JujuCACertEnvKey)
	}
}
