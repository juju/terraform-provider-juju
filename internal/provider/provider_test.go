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
	if v := os.Getenv("JUJU_USERNAME"); v == "" {
		t.Fatal("JUJU_USERNAME must be set for acceptance tests")
	}
	if v := os.Getenv("JUJU_PASSWORD"); v == "" {
		t.Fatal("JUJU_PASSWORD must be set for acceptance tests")
	}
	if v := os.Getenv("JUJU_CA_CERT"); v == "" {
		t.Fatal("JUJU_CA_CERT must be set for acceptance tests")
	}
}
