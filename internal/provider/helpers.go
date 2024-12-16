// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// model names for logging
// @module=juju.<subsystem>
// e.g.:
//
//	@module=juju.resource-application
const (
	LogDataSourceApplication = "datasource-application"
	LogDataSourceMachine     = "datasource-machine"
	LogDataSourceModel       = "datasource-model"
	LogDataSourceOffer       = "datasource-offer"
	LogDataSourceSecret      = "datasource-secret"

	LogResourceApplication     = "resource-application"
	LogResourceAccessModel     = "resource-access-model"
	LogResourceAccessOffer     = "resource-access-offer"
	LogResourceCredential      = "resource-credential"
	LogResourceKubernetesCloud = "resource-kubernetes-cloud"
	LogResourceMachine         = "resource-machine"
	LogResourceModel           = "resource-model"
	LogResourceOffer           = "resource-offer"
	LogResourceSSHKey          = "resource-sshkey"
	LogResourceUser            = "resource-user"
	LogResourceSecret          = "resource-secret"
	LogResourceAccessSecret    = "resource-access-secret"

	LogDataSourceJAASGroup = "datasource-jaas-group"
	LogDataSourceJAASRole  = "datasource-jaas-role"

	LogResourceJAASAccessModel      = "resource-jaas-access-model"
	LogResourceJAASAccessCloud      = "resource-jaas-access-cloud"
	LogResourceJAASAccessGroup      = "resource-jaas-access-group"
	LogResourceJAASAccessRole       = "resource-jaas-access-role"
	LogResourceJAASAccessOffer      = "resource-jaas-access-offer"
	LogResourceJAASAccessController = "resource-jaas-access-controller"
	LogResourceJAASAccessSvcAcc     = "resource-jaas-access-service-account"
	LogResourceJAASGroup            = "resource-jaas-group"
	LogResourceJAASRole             = "resource-jaas-role"
)

const LogResourceIntegration = "resource-integration"

func addClientNotConfiguredError(diag *diag.Diagnostics, resource, method string) {
	diag.AddError(
		"Provider Error, Client Not Configured",
		fmt.Sprintf("Unable to %s %s resource. Expected configured Juju Client. "+
			"Please report this issue to the provider developers.", method, resource),
	)
}

func addDSClientNotConfiguredError(diag *diag.Diagnostics, dataSource string) {
	diag.AddError(
		"Provider Error, Client Not Configured",
		fmt.Sprintf("Unable to read data source %s. Expected configured Juju Client. "+
			"Please report this issue to the provider developers.", dataSource),
	)
}

func intPtr(value types.Int64) *int {
	count := int(value.ValueInt64())
	return &count
}
