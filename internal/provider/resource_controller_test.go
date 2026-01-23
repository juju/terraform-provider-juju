// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/api/connector"
	controllerapi "github.com/juju/juju/api/controller/controller"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceController(t *testing.T) {
	controllerName := acctest.RandomWithPrefix("tf-test-controller")

	mockCtrl := gomock.NewController(t)
	mockJujuCommand := NewMockJujuCommand(mockCtrl)
	defer mockCtrl.Finish()

	mockJujuCommand.EXPECT().Bootstrap(gomock.Any(), juju.BootstrapArguments{
		Name:       controllerName,
		JujuBinary: "/snap/bin/juju",
		Cloud: juju.BootstrapCloudArgument{
			Name:           testingCloud.CloudName(),
			AuthTypes:      []string{"certificate"},
			CACertificates: []string{"test ca cert"},
			Config: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Endpoint: "https://test-endpoint.local",
			Region: &juju.BootstrapCloudRegionArgument{
				Name:             "local",
				Endpoint:         "https://test-endpoint.local/local",
				IdentityEndpoint: "https://test-endpoint.local/local/identity",
				StorageEndpoint:  "https://test-endpoint.local/local/storage",
			},
			Type: "test-type",
		},
		CloudCredential: juju.BootstrapCredentialArgument{
			Name:     "lxd-credentials",
			AuthType: "certificate",
			Attributes: map[string]string{
				"client_cert": "test client cert",
				"client_key":  "test client key",
				"ca_cert":     "test ca cert",
			},
		},
		Config: juju.BootstrapConfig{
			BootstrapConfig: map[string]string{
				"controller_service_type":          "Loadbalancer",
				"controller_external_name":         "test-external-name",
				"controller_external_ip_addresses": "[\"127.0.0.1\", \"127.0.0.2\"]",
			},
			ControllerConfig: map[string]string{
				"agent-logfile-max-backups": "3",
				"audit-log-capture-args":    "true",
				"autocert-dns-name":         "test-external-name",
			},
			ControllerModelConfig: map[string]string{
				"enable-os-refresh-update": "false",
				"http-proxy":               "fake-proxy",
			},
		},
		Flags: juju.BootstrapFlags{
			AgentVersion:  "3.6.12",
			BootstrapBase: "test-base",
		},
	}).Return(&juju.ControllerConnectionInformation{
		Addresses: []string{"127.0.0.1:17070"},
		CACert:    "test controller CA cert",
		Username:  "admin",
		Password:  "password",
	}, nil).AnyTimes()

	mockJujuCommand.EXPECT().Config(
		gomock.Any(),
		&juju.ControllerConnectionInformation{
			Addresses: []string{"127.0.0.1:17070"},
			CACert:    "test controller CA cert",
			Username:  "admin",
			Password:  "password",
		},
	).Return(map[string]any{
		"agent-logfile-max-backups": "3",
		"audit-log-capture-args":    "true",
		"autocert-dns-name":         "test-external-name",
	}, map[string]any{
		"enable-os-refresh-update": "false",
		"http-proxy":               "fake-proxy",
	}, nil).AnyTimes()

	mockJujuCommand.EXPECT().Destroy(
		gomock.Any(),
		&juju.ControllerConnectionInformation{
			Addresses: []string{"127.0.0.1:17070"},
			CACert:    "test controller CA cert",
			Username:  "admin",
			Password:  "password",
		},
	).Return(nil).AnyTimes()

	frameworkProviderFactoriesWithMockJujuCommand := map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(
			NewJujuProvider("dev", ProviderConfiguration{
				WaitForResources: false,
				NewJujuCommand:   func(_ string) (JujuCommand, error) { return mockJujuCommand, nil },
			}),
		),
	}

	resourceName := "juju_controller.controller"
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: frameworkProviderFactoriesWithMockJujuCommand,
		Steps: []resource.TestStep{{
			Config: testAccResourceController(controllerName, testingCloud.CloudName()),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "name", controllerName),
			),
		}},
	})
}

func testAccResourceController(controllerName, cloudName string) string {
	return fmt.Sprintf(`
provider "juju" {
  controller_mode = true
}

resource "juju_controller" "controller" {
  agent_version = "3.6.12"
  name          = %q

  juju_binary     = "/snap/bin/juju"
  bootstrap_base  = "test-base"
  
  bootstrap_config = {
	"controller_service_type"          = "Loadbalancer"
	"controller_external_name"         = "test-external-name"
    "controller_external_ip_addresses" = "[\"127.0.0.1\", \"127.0.0.2\"]"
  }

  controller_config = {
  	"agent-logfile-max-backups" = "3"
	"audit-log-capture-args"    = "true"
	"autocert-dns-name"         = "test-external-name"
  }

  controller_model_config = {
	"enable-os-refresh-update" = "false"
	"http-proxy"               = "fake-proxy"
  }

  cloud = {
    name   = %q
	auth_types = ["certificate"]
	ca_certificates = ["test ca cert"]
	config = {
	  "key1" = "value1"
	  "key2" = "value2"
	}
	endpoint = "https://test-endpoint.local"

	region = {
	  name              = "local"
	  endpoint          = "https://test-endpoint.local/local"
	  storage_endpoint  = "https://test-endpoint.local/local/storage"
	  identity_endpoint = "https://test-endpoint.local/local/identity"
	}

	type   = "test-type"
  } 

  cloud_credential = {
	name = "lxd-credentials"

	auth_type = "certificate"

	attributes = {
		client_cert = "test client cert"
		client_key  = "test client key"
		ca_cert     = "test ca cert"
	}
  }
}
`, controllerName, cloudName)
}

func TestBuildStringListFromMap(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected []string
	}{
		{
			name:     "empty map",
			input:    map[string]string{},
			expected: []string{},
		},
		{
			name: "single entry",
			input: map[string]string{
				"key1": "value1",
			},
			expected: []string{"key1=value1"},
		},
		{
			name: "multiple entries",
			input: map[string]string{
				"key1": "value1",
				"key2": "value2",
				"key3": "value3",
			},
			expected: []string{"key1=value1", "key2=value2", "key3=value3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildStringListFromMap(tt.input)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

// These tests require an LXD or MicroK8s config file to exist at the known path.
// Check `project-docs/BOOTSTRAP_TESTS.md` for more details
// on how to set up the environment.

func TestAcc_ResourceControllerWithJujuBinary(t *testing.T) {
	SkipJAAS(t)
	controllerName := acctest.RandomWithPrefix("tf-test-controller")
	resourceName := "juju_controller.controller"

	// bootstrap config
	baseBootstrapConfig := map[string]string{
		"admin-secret": "my-favorite-admin-password",
	}

	// controller config
	baseControllerConfig := map[string]string{
		"agent-logfile-max-backups": "3",
	}
	updatedControllerConfig := map[string]string{
		"agent-logfile-max-backups": "4",
	}
	unsetControllerConfig := map[string]string{}
	invalidControllerConfig := map[string]string{
		"agent-logfile-max-backups": "3",
		"fake-config-key":           "a value",
	}

	// controller-model config
	baseControllerModelConfig := map[string]string{
		"disable-telemetry": "true",
	}
	updatedControllerModelConfig := map[string]string{
		"disable-telemetry": "true",
		"http-proxy":        "http://my-proxy.local:8080",
	}
	unsetControllerModelConfig := map[string]string{
		"disable-telemetry": "true",
	}

	frameworkProviderFactoriesControllerMode := map[string]func() (tfprotov6.ProviderServer, error){
		"juju": providerserver.NewProtocol6WithError(
			NewJujuProvider("dev", ProviderConfiguration{
				WaitForResources: false,
				NewJujuCommand: func(binaryPath string) (JujuCommand, error) {
					return juju.NewDefaultJujuCommand(binaryPath)
				},
			}),
		),
	}
	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: frameworkProviderFactoriesControllerMode,
		Steps: []resource.TestStep{
			{
				// Step 1: Create the controller
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, baseControllerConfig, baseControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", controllerName),
					resource.TestCheckResourceAttr(resourceName, "bootstrap_config.admin-secret", "my-favorite-admin-password"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.agent-logfile-max-backups", "3"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.disable-telemetry", "true"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"), // 1 element in map
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "1"),
				),
			},
			{
				// Step 2: Verify changing controller config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, updatedControllerConfig, baseControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_config.agent-logfile-max-backups", "4"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "1"),
				),
			},
			{
				// Step 3: Verify unsetting a controller config value behaves as expected.
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, baseControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					func(s *terraform.State) error {
						// Check with Juju that the controller config is the same - Juju doesn't support
						// unsetting config keys, so the previous value should still be present.
						conn, err := newBootstrappedControllerClient(s)
						if err != nil {
							return fmt.Errorf("failed to create controller client: %w", err)
						}
						controllerClient := controllerapi.NewClient(conn)
						configValues, err := controllerClient.ControllerConfig()
						if err != nil {
							return fmt.Errorf("failed to get controller config via Juju API: %w", err)
						}
						// Stringify value as it comes back as interface{} (float64)
						gotValue := fmt.Sprintf("%v", configValues["agent-logfile-max-backups"])
						if gotValue != "4" {
							return fmt.Errorf("expected controller config 'agent-logfile-max-backups' to still be '4', got %q", gotValue)
						}
						return nil
					},
				),
			},
			{
				// Step 4: Verify changing controller model config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, updatedControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.disable-telemetry", "true"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.http-proxy", "http://my-proxy.local:8080"),
				),
			},
			{
				// Step 5: Verify unsetting controller model config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, unsetControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.disable-telemetry", "true"),
					func(s *terraform.State) error {
						// Check with Juju that the controller model config is actually unset.
						conn, err := newBootstrappedControllerClient(s)
						if err != nil {
							return fmt.Errorf("failed to create controller client: %w", err)
						}
						modelCfgClient := modelconfig.NewClient(conn)
						configValues, err := modelCfgClient.ModelGet()
						if err != nil {
							return fmt.Errorf("failed to get controller config via Juju API: %w", err)
						}
						if configValues["http-proxy"] != "" {
							return fmt.Errorf("expected empty value for 'http-proxy' , got %q", configValues["http-proxy"])
						}
						return nil
					},
				),
			},
			{
				// Step 6: Verify that invalid controller config fails
				Config:      testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, invalidControllerConfig, unsetControllerModelConfig),
				ExpectError: regexp.MustCompile("failed to update controller config: unknown controller config"),
			},
		},
	})
}

func testAccResourceControllerWithJujuBinary(controllerName string, bootstrapConfig, controllerConfig, modelConfig map[string]string) string {
	bootstrapConfigHCL := renderStringMapAsHCL(bootstrapConfig)
	controllerConfigHCL := renderStringMapAsHCL(controllerConfig)
	modelConfigHCL := renderStringMapAsHCL(modelConfig)
	switch testingCloud {
	case LXDCloudTesting:
		return fmt.Sprintf(`
provider "juju" {
  controller_mode = true
}

locals {
  lxd_creds = yamldecode(file("~/lxd-credentials.yaml"))
}

resource "juju_controller" "controller" {
  name          = %q

  juju_binary     = "/snap/juju/current/bin/juju"

  bootstrap_config = %s

  controller_config = %s

  controller_model_config = %s

  // Specifying the cloud name as 'localhost' uses the local LXD cloud
  // without the need to specify a cloud endpoint.
  cloud = {
    name   = "localhost"
	auth_types = ["certificate"]
	type = "lxd"
  } 

  cloud_credential = {
	name = "test-credential"
	auth_type = "certificate"
	
	attributes = {
      client-cert = local.lxd_creds.client-cert
      client-key  = local.lxd_creds.client-key
      server-cert = local.lxd_creds.server-cert
    }
  }
  
}
`, controllerName, bootstrapConfigHCL, controllerConfigHCL, modelConfigHCL)
	case MicroK8sTesting:
		return fmt.Sprintf(`
provider "juju" {
  controller_mode = true
}

locals {
  microk8s_config = yamldecode(file("~/microk8s-config.yaml"))
}

resource "juju_controller" "controller" {
  name          = %q

  juju_binary     = "/snap/juju/current/bin/juju"

  bootstrap_config = %s

  controller_config = %s

  controller_model_config = %s

  cloud = {
    name   = "test-k8s"
	auth_types = ["certificate"]
	type = "kubernetes"
	endpoint = local.microk8s_config.clusters[0].cluster.server
	ca_certificates = [base64decode(local.microk8s_config.clusters[0].cluster["certificate-authority-data"])]
	config = {
	   "workload-storage" = "microk8s-hostpath"
	   "operator-storage" = "microk8s-hostpath"
	}
	host_cloud_region = "localhost"
  } 

  cloud_credential = {
	name = "test-credential"
	auth_type = "clientcertificate"
	
	attributes = {
      ClientCertificateData = base64decode(local.microk8s_config.users[0].user["client-certificate-data"])
      ClientKeyData  = base64decode(local.microk8s_config.users[0].user["client-key-data"])
	}
  }
}
`, controllerName, bootstrapConfigHCL, controllerConfigHCL, modelConfigHCL)
	}
	return ""
}

func renderStringMapAsHCL(values map[string]string) string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("{\n")
	for _, k := range keys {
		v := values[k]
		fmt.Fprintf(&b, "%q = %q\n", k, v)
	}
	b.WriteString("}\n")
	return b.String()
}

func newBootstrappedControllerClient(state *terraform.State) (api.Connection, error) {
	resourceState, ok := state.RootModule().Resources["juju_controller.controller"]
	if !ok {
		return nil, fmt.Errorf("resource juju_controller.controller not found in state")
	}

	controllerAddrs, err := getStringListFromTerraformState(resourceState.Primary.Attributes, "api_addresses")
	if err != nil {
		return nil, fmt.Errorf("failed to get controller api_addresses from resource state: %w", err)
	}

	controllerCACert, ok := resourceState.Primary.Attributes["ca_cert"]
	if !ok {
		return nil, fmt.Errorf("ca_cert attribute not found in resource state")
	}

	controllerUsername, ok := resourceState.Primary.Attributes["username"]
	if !ok {
		return nil, fmt.Errorf("username attribute not found in resource state")
	}

	controllerPassword, ok := resourceState.Primary.Attributes["password"]
	if !ok {
		return nil, fmt.Errorf("password attribute not found in resource state")
	}

	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: controllerAddrs,
		CACert:              controllerCACert,
		Username:            controllerUsername,
		Password:            controllerPassword,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create connector to controller: %w", err)
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to controller: %w", err)
	}

	return conn, nil
}

func getStringListFromTerraformState(attrs map[string]string, attrName string) ([]string, error) {
	// A list attributes in state is expecteed to show up in Terraform's flatmap form:
	//   <attr>.# = N
	//   <attr>.0 = ...
	//   <attr>.1 = ...
	countStr, ok := attrs[attrName+".#"]
	if !ok {
		return nil, fmt.Errorf("attribute %q not found in state", attrName+".#")
	}

	count, err := strconv.Atoi(countStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %q count from state: %w", attrName, err)
	}

	out := make([]string, 0, count)
	for i := range count {
		key := fmt.Sprintf("%s.%d", attrName, i)
		v, ok := attrs[key]
		if !ok {
			return nil, fmt.Errorf("attribute %q missing element %d", attrName, i)
		}
		out = append(out, v)
	}

	return out, nil
}
