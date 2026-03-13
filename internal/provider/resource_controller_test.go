// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/connector"
	controllerapi "github.com/juju/juju/api/controller/controller"
	"github.com/juju/names/v6"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/version/v2"
)

func TestAcc_ResourceController(t *testing.T) {
	SkipJAAS(t)
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
		Addresses:    []string{"127.0.0.1:17070"},
		CACert:       "test controller CA cert",
		Username:     "admin",
		Password:     "password",
		AgentVersion: "3.6.12",
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
		juju.DestroyArguments{
			Name:        controllerName,
			JujuBinary:  "/snap/bin/juju",
			CloudName:   testingCloud.CloudName(),
			CloudRegion: "local",
			ConnectionInfo: juju.ControllerConnectionInformation{
				Addresses:    []string{"127.0.0.1:17070"},
				CACert:       "test controller CA cert",
				Username:     "admin",
				Password:     "password",
				AgentVersion: "3.6.12",
			},
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
		"enable-os-upgrade": "false",
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
		Steps: append([]resource.TestStep{
			{
				// Create the controller
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
				// Verify changing controller config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, updatedControllerConfig, baseControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_config.agent-logfile-max-backups", "4"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "1"),
				),
			},
			{
				// Test import using identity with controller_config and controller_model_config set.
				Config:          testAccResourceControllerWithJujuBinaryImport(controllerName),
				ResourceName:    resourceName,
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				// Expect a non-empty plan after import because we need to set connection info and other
				// field in the state. we test that we don't require-replace.
				ExpectNonEmptyPlan: true,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(resourceName, plancheck.ResourceActionUpdate),
					},
				},
			},
			{
				// Verify unsetting a controller config value behaves as expected.
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, baseControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					func(s *terraform.State) error {
						// Check with Juju that the controller config is the same - Juju doesn't support
						// unsetting config keys, so the previous value should still be present.
						conn, err := newBootstrappedControllerClient(t.Context(), s)
						if err != nil {
							return fmt.Errorf("failed to create controller client: %w", err)
						}
						controllerClient := controllerapi.NewClient(conn)
						configValues, err := controllerClient.ControllerConfig(t.Context())
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
				// Verify changing controller model config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, updatedControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "2"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.disable-telemetry", "true"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.enable-os-upgrade", "false"),
				),
			},
			{
				// Verify unsetting controller model config works
				Config: testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, unsetControllerConfig, unsetControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.%", "1"),
					resource.TestCheckResourceAttr(resourceName, "controller_config.%", "0"),
					resource.TestCheckResourceAttr(resourceName, "controller_model_config.disable-telemetry", "true"),
					func(s *terraform.State) error {
						// Check with Juju that the controller model config is actually unset.
						conn, err := newBootstrappedControllerClient(t.Context(), s)
						if err != nil {
							return fmt.Errorf("failed to create controller client: %w", err)
						}
						modelCfgClient := modelconfig.NewClient(conn)
						configValues, err := modelCfgClient.ModelGet(t.Context())
						if err != nil {
							return fmt.Errorf("failed to get controller config via Juju API: %w", err)
						}
						if configValues["enable-os-upgrade"] != true {
							return fmt.Errorf("expected true value for 'enable-os-upgrade' , got %q", configValues["enable-os-upgrade"])
						}
						return nil
					},
				),
			},
			{
				// enable-ha is skipped for k8s controllers, and juju 4.
				SkipFunc: func() (bool, error) {
					if testingCloud != LXDCloudTesting {
						return true, nil
					}
					version, err := TestClient.Applications.GetControllerVersion(t.Context())
					if err != nil {
						t.Fatalf("failed to get controller version: %v", err)
						return true, nil
					}
					if version.Major > 3 {
						return true, nil
					}
					return false, nil
				},
				Config: testAccResourceControllerWithEnableHA(controllerName, baseBootstrapConfig, unsetControllerConfig, unsetControllerModelConfig),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", controllerName),
					func(s *terraform.State) error {
						// Here we check that the enable-HA action has successfully added 3 controller units by checking the WantsVote field via the API.
						conn, err := newBootstrappedControllerClient(t.Context(), s)
						if err != nil {
							return fmt.Errorf("failed to create controller client for HA check: %w", err)
						}
						defer conn.Close()

						// Find the controller model UUID.
						models, err := controllerapi.NewClient(conn).AllModels(t.Context())
						if err != nil {
							return fmt.Errorf("failed to list models for HA check: %w", err)
						}
						var controllerModelUUID string
						for _, m := range models {
							if m.Name == "controller" {
								controllerModelUUID = m.UUID
								break
							}
						}
						if controllerModelUUID == "" {
							return fmt.Errorf("controller model not found in AllModels response")
						}

						// ModelInfo returns machine details including WantsVote for HA nodes.
						results, err := modelmanager.NewClient(conn).ModelInfo(
							t.Context(),
							[]names.ModelTag{names.NewModelTag(controllerModelUUID)},
						)
						if err != nil {
							return fmt.Errorf("failed to get controller model info for HA check: %w", err)
						}
						if len(results) == 0 || results[0].Error != nil {
							return fmt.Errorf("unexpected model info result: %v", results)
						}

						// WantsVote is set immediately on all controller nodes when
						// EnableHA is requested, before machines finish provisioning.
						if len(results[0].Result.Machines) != 3 {
							return fmt.Errorf("expected 3 controller units for HA, got %d", len(results[0].Result.Machines))
						}
						return nil
					},
				),
			},
			{
				// Verify that invalid controller config fails
				Config:      testAccResourceControllerWithJujuBinary(controllerName, baseBootstrapConfig, invalidControllerConfig, unsetControllerModelConfig),
				ExpectError: regexp.MustCompile("failed to update controller config: unknown controller config"),
			},
		}, testJAASControllerResourceSteps(t, resourceName, controllerName, baseBootstrapConfig)...),
		CheckDestroy: func(s *terraform.State) error {
			if isJAAS() {
				if err := testAccCheckJaasControllerRegistered(t, controllerName, false)(s); err != nil {
					return err
				}
			}

			// Skip this check for 2.9, where it can fail intermittently
			version, err := getAgentVersionFromState(s)
			if err != nil {
				return fmt.Errorf("failed to get agent version from state: %w", err)
			}
			if version.Major == 2 && version.Minor < 9 {
				return nil
			}

			// Attempt to connect and expect a timeout after 10s
			// This doesn't definitely prove the controller is destroyed, but is a good heuristic.
			_, err = newBootstrappedControllerClient(t.Context(), s, api.WithDialOpts(api.DialOpts{Timeout: 10 * time.Second}))
			if err != nil {
				if ok, err := regexp.MatchString("failed to connect to controller: .*", err.Error()); ok && err == nil {
					return nil
				}
				return fmt.Errorf("unexpected error when connecting to detroyed controller: %w", err)
			} else {
				return fmt.Errorf("unexpectedly managed to connect to controller")
			}
		},
	})
}

func lxdBridgeIPv4Address() (string, error) {
	// Equivalent of: lxc network get lxdbr0 ipv4.address | cut -f1 -d/
	cmd := exec.Command("lxc", "network", "get", "lxdbr0", "ipv4.address")
	outBytes, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("running %q failed: %w", strings.Join(cmd.Args, " "), err)
	}
	out := strings.TrimSpace(string(outBytes))
	if out == "" {
		return "", fmt.Errorf("empty output")
	}
	ip, _, _ := strings.Cut(out, "/")
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return "", fmt.Errorf("unexpected output %q", out)
	}
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("unexpected non-IP output %q", out)
	}
	return ip, nil
}

func buildJaasCloudInitUserdata(jaasHost string, caCert string) (string, error) {
	if strings.TrimSpace(jaasHost) == "" {
		return "", fmt.Errorf("empty JAAS host")
	}
	if strings.TrimSpace(caCert) == "" {
		return "", fmt.Errorf("empty CA cert")
	}
	caCert = strings.TrimSpace(caCert)
	caCert = strings.TrimSuffix(caCert, "\n")
	caCertIndented := indentLines(caCert, "      ")

	hostIP, err := lxdBridgeIPv4Address()
	if err != nil {
		return "", err
	}

	var b strings.Builder
	// Adds a hosts entry so the container can resolve the JAAS endpoint.
	b.WriteString("preruncmd:\n")
	fmt.Fprintf(&b, "  - echo \"%s    %s\" >> /etc/hosts\n", hostIP, jaasHost)
	// Installs the CA cert so the controller can validate the JAAS endpoint.
	b.WriteString("ca-certs:\n")
	b.WriteString("  trusted:\n")
	b.WriteString("    - |\n")
	b.WriteString(caCertIndented)
	b.WriteString("\n")
	return b.String(), nil
}

func indentLines(s string, prefix string) string {
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

// testAccResourceControllerWithJujuBinaryImport returns a Terraform configuration
// for a juju_controller resource using the Juju binary for import.
// During import for lxd we need to:
//   - ignore changes to cloud attributes, because the client-key and client-cert
//     values are put in the state by the import but they shouldn't require replace.
//   - ignore changes to cloud.region and cloud.endpoint
//
// During import for microk8s we need to:
//   - ignore changes to cloud.region and cloud.host_cloud_region.
func testAccResourceControllerWithJujuBinaryImport(controllerName string) string {
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
      server-cert = local.lxd_creds.server-cert
    }
  }

  lifecycle {
	ignore_changes = [
	  cloud.endpoint,
	  cloud.region,
	  cloud_credential.attributes["client-cert"],
      cloud_credential.attributes["client-key"]
	]
   }
}
`, controllerName)
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

  cloud = {
    name   = "test-k8s"
	auth_types = ["clientcertificate"]
	type = "kubernetes"
	endpoint = local.microk8s_config.clusters[0].cluster.server
	ca_certificates = [base64decode(local.microk8s_config.clusters[0].cluster["certificate-authority-data"])]
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
  lifecycle {
    ignore_changes = [
	  cloud.region,
	  cloud.host_cloud_region
    ]
  }
}
`, controllerName)
	}
	return ""
}

func testAccResourceControllerWithJujuBinary(controllerName string, bootstrapConfig, controllerConfig, modelConfig map[string]string) string {
	if isJAAS() {
		// If JAAS, set the controller's login-token-refresh-url to JAAS.
		addrs := os.Getenv(JujuControllerEnvKey)
		jaasAddr := strings.TrimSpace(strings.Split(addrs, ",")[0])
		jaasHost, _, err := net.SplitHostPort(jaasAddr)
		if err != nil {
			panic(fmt.Sprintf("invalid %s=%q: %v", JujuControllerEnvKey, addrs, err))
		}
		if jaasHost == "" {
			panic(fmt.Sprintf("invalid %s=%q: no addresses", JujuControllerEnvKey, addrs))
		}
		bootstrapConfig["login-token-refresh-url"] = fmt.Sprintf("https://%s/.well-known/jwks.json", jaasHost)

		// Ensure the controller can reach JAAS and trust its public key for controller registration.
		//
		// - Ensure JAAS DNS resolves from inside the controller.
		// - Trust the CA used by JAAS.
		if testingCloud != LXDCloudTesting {
			panic("testing controller bootstrap with JAAS without LXD is not supported")
		}
		cloudInit, err := buildJaasCloudInitUserdata(jaasHost, os.Getenv(JujuCACertEnvKey))
		if err != nil {
			panic(fmt.Sprintf("failed to build JAAS cloud-init userdata: %v", err))
		}
		bootstrapConfig["cloudinit-userdata"] = cloudInit
	}
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

  bootstrap_constraints = {
    "cores"            = "2"
    "mem"              = "4G"
    "root-disk"        = "4G"
  }

  bootstrap_config        = %s
  controller_config       = %s
  controller_model_config = %s

  destroy_flags	= {
	destroy_all_models = true
  }

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
      server-cert = local.lxd_creds.server-cert
	  client-key = local.lxd_creds.client-key
	  client-cert = local.lxd_creds.client-cert
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

// testAccResourceControllerWithEnableHA returns HCL that bootstraps a controller
// and runs the juju_enable_ha action with 3 units.
func testAccResourceControllerWithEnableHA(controllerName string, bootstrapConfig, controllerConfig, modelConfig map[string]string) string {
	base := testAccResourceControllerWithJujuBinary(controllerName, bootstrapConfig, controllerConfig, modelConfig)
	return base + `
resource "terraform_data" "test" {
  lifecycle {
    action_trigger {
      events  = [after_create]
      actions = [action.juju_enable_ha.ctrl_ha]
    }
  }
}

action "juju_enable_ha" "ctrl_ha" {
  config {
  	api_addresses = juju_controller.controller.api_addresses
  	ca_cert       = juju_controller.controller.ca_cert
  	username      = juju_controller.controller.username
  	password      = juju_controller.controller.password
  	units         = 3
  }
}
`
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

func newBootstrappedControllerClient(ctx context.Context, state *terraform.State, dialOptions ...api.DialOption) (api.Connection, error) {
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

	conn, err := connr.Connect(ctx, dialOptions...)
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

func getAgentVersionFromState(s *terraform.State) (*version.Number, error) {
	resourceState, ok := s.RootModule().Resources["juju_controller.controller"]
	if !ok {
		return nil, fmt.Errorf("resource juju_controller.controller not found in state")
	}

	agentVersion, ok := resourceState.Primary.Attributes["agent_version"]
	if !ok {
		return nil, fmt.Errorf("ca_cert attribute not found in resource state")
	}

	parsedVersion, err := version.Parse(agentVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to parse agent_version %q: %w", agentVersion, err)
	}

	return &parsedVersion, nil
}

func testAccCheckJaasControllerRegistered(t *testing.T, name string, checkExists bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		testAccPreCheck(t)
		if TestClient == nil {
			return fmt.Errorf("TestClient is not set")
		}

		controllers, err := TestClient.Jaas.ListControllers(t.Context())
		if err != nil {
			return err
		}

		found := false
		for _, c := range controllers {
			if c.Name == name {
				found = true
				break
			}
		}

		if checkExists && !found {
			return fmt.Errorf("expected controller %q to be registered", name)
		}
		if !checkExists && found {
			return fmt.Errorf("controller %q still registered", name)
		}
		return nil
	}
}
