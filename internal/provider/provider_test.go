// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

const (
	TestProviderStableVersion = "0.21.1"
	isJaasEnvKey              = "IS_JAAS"
)

// providerFactories are used to instantiate the Framework provider during
// acceptance testing.
var frameworkProviderFactories map[string]func() (tfprotov6.ProviderServer, error)

// frameworkProviderFactoriesNoResourceWait are used to instantiate the Framework provider during
// acceptance testing but configures the provider to not wait for resources to be ready or destroyed.
var frameworkProviderFactoriesNoResourceWait map[string]func() (tfprotov6.ProviderServer, error)

// Provider makes a separate provider available for tests.
// Note that testAccPreCheck needs to invoked before use.
var Provider provider.Provider

// TestClient is needed for any resource to be able to use Juju client in
// custom checkers for their tests (e.g. resource_model_test)
var TestClient *juju.Client

// setupAccTestsOnce ensures that any setup needed for acceptance tests
// is only done once.
var setupAccTestsOnce sync.Once

func init() {
	waitForResources := true
	Provider = NewJujuProvider("dev", waitForResources)

	frameworkProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev", true)),
	}
	frameworkProviderFactoriesNoResourceWait = map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(NewJujuProvider("dev", false)),
	}
}

// SkipJAAS should be called at the top of any tests that are not appropriate to
// run against JAAS. These include things like Juju access related tests where a
// JAAS specific resource is available.
func SkipJAAS(t *testing.T) {
	if _, ok := os.LookupEnv("IS_JAAS"); ok {
		t.Skip("Skipping test when running against JAAS")
	}
}

// OnlyTestAgainstJAAS should be called at the top of any tests that are not
// appropriate to run against a Juju controller. This includes tests for all JAAS
// specific resources where only JAAS implements the necessary API methods.
func OnlyTestAgainstJAAS(t *testing.T) {
	if _, ok := os.LookupEnv("IS_JAAS"); !ok {
		t.Skip("Skipping JAAS specific test against Juju")
	}
}

func TestProviderConfigure(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev", true)
	confResp := configureProvider(t, jujuProvider)
	assert.Equal(t, confResp.Diagnostics.HasError(), false)
}

func TestProviderConfigureUsernameFromEnv(t *testing.T) {
	SkipJAAS(t)
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev", true)
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
	SkipJAAS(t)
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev", true)
	passwordValue := "the-password"
	t.Setenv(JujuPasswordEnvKey, passwordValue)
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "invalid entity name or password (unauthorized access)", err.Detail())
}

func TestProviderConfigureClientIDAndSecretFromEnv(t *testing.T) {
	SkipJAAS(t)
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev", true)
	emptyValue := ""
	t.Setenv(JujuUsernameEnvKey, emptyValue)
	t.Setenv(JujuPasswordEnvKey, emptyValue)

	clientIDValue := "test-client-id"
	t.Setenv(JujuClientIDEnvKey, clientIDValue)
	clientSecretValue := "test-client-secret"
	t.Setenv(JujuClientSecretEnvKey, clientSecretValue)

	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "this version of Juju does not support login from old clients (not supported) (not supported)", err.Detail())
}

func TestProviderConfigureAddresses(t *testing.T) {
	testAccPreCheck(t)
	os.Setenv("JUJU_CONNECTION_TIMEOUT", "2") // 2s timeout
	defer os.Unsetenv("JUJU_CONNECTION_TIMEOUT")
	jujuProvider := NewJujuProvider("dev", true)
	// This IP is from a test network that should never be routed. https://www.rfc-editor.org/rfc/rfc5737#section-3
	t.Setenv(JujuControllerEnvKey, "192.168.1.100:17070")
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "Connection error, please check the controller_addresses property set on the provider", err.Detail())
}

// This is a valid certificate allowing the client to attempt a connection but failing certificate validation
const (
	invalidCA = "-----BEGIN CERTIFICATE-----\nMIIDazCCAlOgAwIBAgIULHtYyq/mjGAaZTTFcfd4Dmi6LtkwDQYJKoZIhvcNAQEL\nBQAwRTELMAkGA1UEBhMCQVUxEzARBgNVBAgMClNvbWUtU3RhdGUxITAfBgNVBAoM\nGEludGVybmV0IFdpZGdpdHMgUHR5IEx0ZDAeFw0yMjA2MjQxNTQzMTFaFw0yMjA3\nMjQxNTQzMTFaMEUxCzAJBgNVBAYTAkFVMRMwEQYDVQQIDApTb21lLVN0YXRlMSEw\nHwYDVQQKDBhJbnRlcm5ldCBXaWRnaXRzIFB0eSBMdGQwggEiMA0GCSqGSIb3DQEB\nAQUAA4IBDwAwggEKAoIBAQCgSrxunimy/Nig3y5mAUtc3quvJI7MVdlWrhhWcNP4\nacF6bsAYDMa02Praf3pUBkyU9Fe83nalcimVO1NO18/FvKK4ZYuwQi4B+Rx1ltF/\nZx5czxrH+kb9FsZJNAtxbAo0hT9rusuCd1m0zhzSOZCTWkmguDew41IQHUtW7Wgy\nM0TlmrCzJkf2w+GwmhxFbJLR37b7N2ylyrFyuLTEKSMAxSw7k4+Djqgat5NdVGmo\niTZST86Br9Xg+goVjFTHxj/f84OaazM6DhyIdizyntkIV6nZVxZmhisO9iWk41Q/\noPeN4ZYUCe+VpZoZShMZ7H281tOYfgCOP2IHyQxxwLQBAgMBAAGjUzBRMB0GA1Ud\nDgQWBBS1ziAYMPkbTHaOfgpKlX70/wkusDAfBgNVHSMEGDAWgBS1ziAYMPkbTHaO\nfgpKlX70/wkusDAPBgNVHRMBAf8EBTADAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQAN\n76z4TTrH5Wj7nPBROyx9Ab3TCF+gSqi2lhxCo5obtdAUdnfsbTtIGH82Ayduz13R\nvWcqn0EXgi2jJ8fMQxujalBwqhw2BPLgXPhIlR8/IcvUp9CIQA3FasvqNrSrfUzJ\ntO9oA3LG5EGnlxeDS5ehkx/bAOQl4yz70Vh+xssU/E5T74Zb8Kgf8uSZbj2jbRh7\nBC4qYzO7jVFOLkIWUjIeKlE2iG3OJnb17NMuODApPLyRslKvRyxwITtWr/jhaTNQ\n4L64mCtPPU2bMLScqsEYDOx237na8m9Xej6MOGb1D4noe59ML/4IwCmG2iK982mQ\n2zpE+UCo97FGq4kDK6bc\n-----END CERTIFICATE-----\n"
)

// TODO: find an alternative way of running test on Mac
func TestProviderConfigurex509FromEnv(t *testing.T) {
	SkipJAAS(t)
	if runtime.GOOS == "darwin" {
		//Due to a bug in Go this test does not work on darwin OS
		//https://github.com/golang/go/issues/52010
		t.Skip("This test does not work on MacOS")
	}
	jujuProvider := NewJujuProvider("dev", true)
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
	SkipJAAS(t)
	jujuProvider := NewJujuProvider("dev", true)
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

func TestProviderAllowsEmptyCACert(t *testing.T) {
	SkipJAAS(t)
	jujuProvider := NewJujuProvider("dev", true)
	//Set the CA cert to be empty and check that the provider still tries to connect.
	t.Setenv(JujuCACertEnvKey, "")
	t.Setenv("JUJU_CA_CERT_FILE", "")
	confResp := configureProvider(t, jujuProvider)
	// This is a live test, expect that the client connection will fail.
	assert.Equal(t, confResp.Diagnostics.HasError(), true)
	err := confResp.Diagnostics.Errors()[0]
	assert.Equal(t, diag.SeverityError, err.Severity())
	assert.Equal(t, "The ca_certificate provider property is not set and the Juju certificate authority is not trusted by your system", err.Detail())
	assert.Equal(t, "x509: certificate signed by unknown authority", err.Summary())
}

func TestProviderSetWarnOnDeletionErrors(t *testing.T) {
	jujuProvider := NewJujuProvider("dev", true)
	confResp := configureProvider(t, jujuProvider)
	providerData, ok := confResp.ResourceData.(juju.ProviderData)
	require.Truef(t, ok, "ResourceData, not of type juju ProviderData")
	require.NotNil(t, providerData)

	assert.Equal(t, providerData.Config.SkipFailedDeletion, false)

	t.Setenv(SkipFailedDeletionEnvKey, "true")
	confResp = configureProvider(t, jujuProvider)
	providerData, ok = confResp.ResourceData.(juju.ProviderData)
	require.Truef(t, ok, "ResourceData, not of type juju client")
	require.NotNil(t, providerData)

	assert.Equal(t, providerData.Config.SkipFailedDeletion, true)
}

func testAccPreCheck(t *testing.T) {
	setupAccTestsOnce.Do(func() {
		setupAcceptanceTests(t)
	})
}

func setupAcceptanceTests(t *testing.T) {
	if TestClient != nil {
		return
	}
	if val, ok := os.LookupEnv(isJaasEnvKey); ok && val == "true" {
		validateJAASTestConfig(t)
	} else {
		validateJujuTestConfig(t)
	}
	confResp := configureProvider(t, Provider)
	require.Equal(t, confResp.Diagnostics.HasError(), false, fmt.Sprintf("provider configuration failed: %v", confResp.Diagnostics.Errors()))
	testClient, ok := confResp.ResourceData.(*juju.Client)
	require.Truef(t, ok, "ResourceData, not of type juju client")
	TestClient = testClient
	createCloudCredential(t)
}

var cloudNameMap = map[string]string{
	"lxd": "localhost",
}

// canonicalCloudName returns the canonical name of the cloud.
// Where the Terraform provider tests uses the name "lxd" for the
// local cloud, the Juju client uses "localhost" for example.
func canonicalCloudName(name string) string {
	val, ok := cloudNameMap[name]
	if ok {
		return val
	}
	return name
}

// createCloudCredential reads a cloud-credential from
// the local client store and re-creates that cloud-credential
// on the controller. If a cloud-credential for the cloud
// under test already exists on the controller, this is a no-op.
// This is useful when running tests as a user that isn't the admin user.
func createCloudCredential(t *testing.T) {
	if TestClient == nil {
		t.Fatal("TestClient is not set")
	}
	cloudName := canonicalCloudName(os.Getenv(TestCloudEnvKey))

	// List controller credentials to bail out early if one already exists.
	controllerCreds, _ := TestClient.Credentials.ListControllerCredentials()
	// skip checking the error here, because the controller
	// returns a not found error if no credentials exist
	// and for any other errors we want to continue anyway.
	if controllerCreds != nil {
		for cloud := range controllerCreds.CloudCredentials {
			if cloud == cloudName {
				t.Logf("successfully found cloud-credential in controller for cloud %s", cloudName)
				return
			}
		}
	}

	// List client credentials to check if we have any cloud-credentials.
	clientCreds, err := TestClient.Credentials.ListClientCredentials()
	if err != nil {
		t.Fatalf("failed to read cloud-credential from client store: %v", err)
	}
	if len(clientCreds.CloudCredentials[cloudName].AuthCredentials) == 0 {
		t.Fatalf("no cloud-credentials for cloud %q found in client store", cloudName)
	}

	// Create a new credential on the controller using the
	// first available client credential.
	var createCredential juju.CreateCredentialInput
	for credentialName, cred := range clientCreds.CloudCredentials[cloudName].AuthCredentials {
		createCredential.AuthType = string(cred.AuthType())
		createCredential.Attributes = cred.Attributes()
		createCredential.Name = credentialName
		createCredential.CloudName = cloudName
		createCredential.ControllerCredential = true
		break
	}

	_, err = TestClient.Credentials.CreateCredential(createCredential)
	if err != nil {
		t.Fatalf("failed to create controller credential: %v", err)
	}
}

func validateJAASTestConfig(t *testing.T) {
	if v := os.Getenv(JujuClientIDEnvKey); v == "" {
		t.Fatalf("%s must be set for acceptance tests", JujuClientIDEnvKey)
	}
	if v := os.Getenv(JujuClientSecretEnvKey); v == "" {
		t.Fatalf("%s must be set for acceptance tests", JujuClientSecretEnvKey)
	}
}

func validateJujuTestConfig(t *testing.T) {
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

func configureProvider(t *testing.T, p provider.Provider) provider.ConfigureResponse {
	conf := jujuProviderModel{}
	confReq := newConfigureRequest(t, conf)
	confResp := provider.ConfigureResponse{Diagnostics: diag.Diagnostics{}}

	p.Configure(context.Background(), confReq, &confResp)

	return confResp
}

func newConfigureRequest(t *testing.T, conf jujuProviderModel) provider.ConfigureRequest {
	schemaResp := provider.SchemaResponse{}
	Provider.Schema(context.Background(), provider.SchemaRequest{}, &schemaResp)
	assert.Equal(t, schemaResp.Diagnostics.HasError(), false)

	mapTypes := map[string]attr.Type{
		JujuController:     types.StringType,
		JujuUsername:       types.StringType,
		JujuPassword:       types.StringType,
		JujuCACert:         types.StringType,
		JujuClientID:       types.StringType,
		JujuClientSecret:   types.StringType,
		SkipFailedDeletion: types.BoolType,
	}

	val, confObjErr := types.ObjectValueFrom(context.Background(), mapTypes, conf)
	assert.Equalf(t, confObjErr.HasError(), false, "failed to create config object: %v", confObjErr)

	tfval, tfValErr := val.ToTerraformValue(context.Background())
	assert.Equal(t, tfValErr, nil)

	c := tfsdk.Config{Schema: schemaResp.Schema, Raw: tfval}
	return provider.ConfigureRequest{Config: c}
}

func TestFrameworkProviderSchema(t *testing.T) {
	testAccPreCheck(t)
	jujuProvider := NewJujuProvider("dev", true)
	req := provider.SchemaRequest{}
	resp := provider.SchemaResponse{}
	jujuProvider.Schema(context.Background(), req, &resp)
	assert.Equal(t, resp.Diagnostics.HasError(), false)
	assert.Len(t, resp.Schema.Attributes, 6)
}

// TestGetJujuProviderModel tests the getJujuProviderModel function.
// Note that getJujuProviderModel falls back to "live discovery" if
// required configuration are missing. This means that testing for
// missing required fields may/may not work depending on the
// environment the tests are run in.
func TestGetJujuProviderModel(t *testing.T) {
	tests := []struct {
		name           string
		plan           jujuProviderModel
		setEnv         func(t *testing.T)
		wantErr        bool
		wantErrSummary string
		wantValues     jujuProviderModel
	}{
		{
			name: "ValidPlanData",
			plan: jujuProviderModel{
				ControllerAddrs:    types.StringValue("localhost:17070"),
				UserName:           types.StringValue("user"),
				Password:           types.StringValue("pass"),
				CACert:             types.StringValue("cert"),
				SkipFailedDeletion: types.BoolValue(true),
			},
			wantErr: false,
			wantValues: jujuProviderModel{
				ControllerAddrs:    types.StringValue("localhost:17070"),
				UserName:           types.StringValue("user"),
				Password:           types.StringValue("pass"),
				CACert:             types.StringValue("cert"),
				SkipFailedDeletion: types.BoolValue(true),
			},
		},
		{
			name: "BothLoginMethodsSet",
			plan: jujuProviderModel{
				ControllerAddrs: types.StringValue("localhost:17070"),
				UserName:        types.StringValue("user"),
				Password:        types.StringValue("pass"),
				ClientID:        types.StringValue("clientid"),
				ClientSecret:    types.StringValue("clientsecret"),
			},
			wantErr:        true,
			wantErrSummary: "Only username and password OR client id and client secret may be used.",
		},
		{
			name: "EnvVarsUsed",
			plan: jujuProviderModel{},
			setEnv: func(t *testing.T) {
				t.Setenv(JujuControllerEnvKey, "env-controller:17070")
				t.Setenv(JujuUsernameEnvKey, "env-user")
				t.Setenv(JujuPasswordEnvKey, "env-pass")
				t.Setenv(JujuCACertEnvKey, "env-cert")
			},
			wantErr: false,
			wantValues: jujuProviderModel{
				ControllerAddrs:    types.StringValue("env-controller:17070"),
				UserName:           types.StringValue("env-user"),
				Password:           types.StringValue("env-pass"),
				CACert:             types.StringValue("env-cert"),
				SkipFailedDeletion: types.BoolValue(false),
			},
		},
		{
			name: "MixPlanAndEnvVars",
			plan: jujuProviderModel{
				ControllerAddrs: types.StringValue("localhost:17070"),
				UserName:        types.StringValue("user"),
				CACert:          types.StringValue("cert"),
			},
			setEnv: func(t *testing.T) {
				t.Setenv(JujuPasswordEnvKey, "env-pass")
				t.Setenv(SkipFailedDeletionEnvKey, "true")
			},
			wantErr: false,
			wantValues: jujuProviderModel{
				ControllerAddrs:    types.StringValue("localhost:17070"),
				UserName:           types.StringValue("user"),
				Password:           types.StringValue("env-pass"),
				CACert:             types.StringValue("cert"),
				SkipFailedDeletion: types.BoolValue(true),
			},
		},
		{
			name: "ConfigOverridesEnvVars",
			plan: jujuProviderModel{
				ControllerAddrs: types.StringValue("localhost:17070"),
				UserName:        types.StringValue("user"),
				Password:        types.StringValue("pass"),
				CACert:          types.StringValue("cert"),
			},
			setEnv: func(t *testing.T) {
				t.Setenv(JujuUsernameEnvKey, "env-user")
				t.Setenv(JujuPasswordEnvKey, "env-pass")
			},
			wantErr: false,
			wantValues: jujuProviderModel{
				ControllerAddrs:    types.StringValue("localhost:17070"),
				UserName:           types.StringValue("user"),
				Password:           types.StringValue("pass"),
				CACert:             types.StringValue("cert"),
				SkipFailedDeletion: types.BoolValue(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setEnv != nil {
				tt.setEnv(t)
			}

			confReq := newConfigureRequest(t, tt.plan)
			model, diags := getJujuProviderModel(context.Background(), confReq)

			if tt.wantErrSummary != "" {
				require.True(t, diags.HasError())
				found := false
				for _, err := range diags.Errors() {
					if err.Summary() == tt.wantErrSummary {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected error for %s", tt.name)
			} else {
				require.False(t, diags.HasError(), "Unexpected error :%v", diags.Errors())
				assert.Equal(t, tt.wantValues, model)
			}
		})
	}
}

func expectedResourceOwner() string {
	// Only 1 field is expected to be populated.
	username := os.Getenv(JujuUsernameEnvKey)
	clientId := os.Getenv(JujuClientIDEnvKey)
	if clientId != "" {
		clientId = clientId + "@serviceaccount"
	}
	return username + clientId
}
