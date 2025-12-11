// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	gomock "go.uber.org/mock/gomock"
)

func TestAcc_ResourceController(t *testing.T) {
	controllerName := acctest.RandomWithPrefix("tf-test-controller")
	outputFile := fmt.Sprintf("/tmp/%s-info.json", controllerName)

	mockCtrl := gomock.NewController(t)
	mockJujuCommand := NewMockjujuCommand(mockCtrl)
	currentDefaultNewJujuCommandFunction := defaultNewJujuCommandFunction
	defaultNewJujuCommandFunction = func(_ string) jujuCommand {
		return mockJujuCommand
	}
	defer func() {
		defaultNewJujuCommandFunction = currentDefaultNewJujuCommandFunction
	}()
	defer mockCtrl.Finish()

	mockJujuCommand.EXPECT().Bootstrap(gomock.Any(), boostrapArguments{
		AgentVersion: "3.6.12",
		Name:         controllerName,
		OutputFile:   outputFile,
		JujuBinary:   "/snap/bin/juju",
		Cloud: bootstrapCloudArgument{
			Name:           testingCloud.CloudName(),
			AuthTypes:      []string{"certificate"},
			CACertificates: []string{"test ca cert"},
			Config: map[string]string{
				"key1": "value1",
				"key2": "value2",
			},
			Endpoint: "https://test-endpoint.local",
			Region: &boostrapCloudRegionArgument{
				Name:             "local",
				Endpoint:         "https://test-endpoint.local/local",
				IdentityEndpoint: "https://test-endpoint.local/local/identity",
				StorageEndpoint:  "https://test-endpoint.local/local/storage",
			},
			Type: "test-type",
		},
		CloudCredential: bootstrapCredentialArgument{
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
	}).Return(&controllerConnectionInformation{
		Addresses: []string{"127.0.0.1:17070"},
		CACert:    "test controller CA cert",
		Username:  "admin",
		Password:  "password",
	}, nil).AnyTimes()

	mockJujuCommand.EXPECT().Config(
		gomock.Any(),
		&controllerConnectionInformation{
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
		&controllerConnectionInformation{
			Addresses: []string{"127.0.0.1:17070"},
			CACert:    "test controller CA cert",
			Username:  "admin",
			Password:  "password",
		},
	).Return(nil).AnyTimes()

	resourceName := "juju_controller.controller"
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: frameworkProviderFactories,
		Steps: []resource.TestStep{{
			Config: testAccResourceController(controllerName, outputFile, testingCloud.CloudName()),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(resourceName, "name", controllerName),
				resource.TestCheckResourceAttr(resourceName, "output_file", outputFile),
			),
		}},
	})
}

func testAccResourceController(controllerName, outputFile, cloudName string) string {
	return fmt.Sprintf(`
resource "juju_controller" "controller" {
  agent_version = "3.6.12"
  name        = %q

  output_file = %q

  juju_binary = "/snap/bin/juju"

  config = {
	"config_key_1" = "config_value_1"
	"config_key_2" = "config_value_2"
  }	

  cloud = {
    name   = %q
	auth_type = ["certificate"]
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
`, controllerName, outputFile, cloudName)
}
