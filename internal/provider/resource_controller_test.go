// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	gomock "go.uber.org/mock/gomock"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func TestAcc_ResourceController(t *testing.T) {
	controllerName := acctest.RandomWithPrefix("tf-test-controller")

	mockCtrl := gomock.NewController(t)
	mockJujuCommand := NewMockJujuCommand(mockCtrl)
	defer mockCtrl.Finish()

	mockJujuCommand.EXPECT().Bootstrap(gomock.Any(), juju.BootstrapArguments{
		AdminSecret:      "test-admin-secret",
		AgentVersion:     "3.6.12",
		BootstrapBase:    "test-base",
		BootstrapTimeout: "15m",
		CAPrivateKey:     "test-ca-private-key",
		Name:             controllerName,
		JujuBinary:       "/snap/bin/juju",
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
		Config: map[string]string{
			"config_key_1": "config_value_1",
			"config_key_2": "config_value_2",
		},
		ControllerExternalIPAddrs: []string{"127.0.0.1", "127.0.0.2"},
		ControllerExternalName:    "test-external-name",
		ControllerServiceType:     "Loadbalancer",
		SSHServerHostKey:          "test-ssh-server-host-key",
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
	).Return(map[string]string{
		"config_key_1": "config_value_1",
		"config_key_2": "config_value_2",
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
				ControllerMode:   true,
				WaitForResources: false,
				NewJujuCommand:   func(_ string) (JujuCommand, error) { return mockJujuCommand, nil },
			}),
		),
	}

	resourceName := "juju_controller.controller"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
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
resource "juju_controller" "controller" {
  agent_version = "3.6.12"
  name        = %q

  juju_binary = "/snap/bin/juju"

  admin_secret = "test-admin-secret"
  controller_external_ip_addresses = ["127.0.0.1", "127.0.0.2"]
  controller_external_name = "test-external-name"
  controller_service_type = "Loadbalancer"
  ssh_server_host_key = "test-ssh-server-host-key"
  ca_private_key = "test-ca-private-key"
  
  bootstrap_timeout = "15m"
  bootstrap_base  = "test-base"

  config = {
	"config_key_1" = "config_value_1"
	"config_key_2" = "config_value_2"
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
	  name     = "local"
	  endpoint = "https://test-endpoint.local/local"
	  identity_endpoint = "https://test-endpoint.local/local/identity"
	  storage_endpoint  = "https://test-endpoint.local/local/storage"
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
