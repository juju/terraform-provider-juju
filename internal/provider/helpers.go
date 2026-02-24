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
	// LogDataSourceApplication is the logging subsystem for application data sources.
	LogDataSourceApplication = "datasource-application"
	// LogDataSourceMachine is the logging subsystem for machine data sources.
	LogDataSourceMachine = "datasource-machine"
	// LogDataSourceModel is the logging subsystem for model data sources.
	LogDataSourceModel = "datasource-model"
	// LogDataSourceOffer is the logging subsystem for offer data sources.
	LogDataSourceOffer = "datasource-offer"
	// LogDataSourceSecret is the logging subsystem for secret data sources.
	LogDataSourceSecret = "datasource-secret"
	// LogDataSourceStoragePool is the logging subsystem for storage pool data sources.
	LogDataSourceStoragePool = "datasource-storage-pool"

	// LogResourceApplication is the logging subsystem for application resources.
	LogResourceApplication = "resource-application"
	// LogResourceAccessModel is the logging subsystem for access model resources.
	LogResourceAccessModel = "resource-access-model"
	// LogResourceAccessOffer is the logging subsystem for access offer resources.
	LogResourceAccessOffer = "resource-access-offer"
	// LogResourceCredential is the logging subsystem for credential resources.
	LogResourceCredential = "resource-credential"
	// LogResourceKubernetesCloud is the logging subsystem for Kubernetes cloud resources.
	LogResourceKubernetesCloud = "resource-kubernetes-cloud"
	// LogResourceMachine is the logging subsystem for machine resources.
	LogResourceMachine = "resource-machine"
	// LogResourceModel is the logging subsystem for model resources.
	LogResourceModel = "resource-model"
	// LogResourceOffer is the logging subsystem for offer resources.
	LogResourceOffer = "resource-offer"
	// LogResourceSSHKey is the logging subsystem for SSH key resources.
	LogResourceSSHKey = "resource-sshkey"
	// LogResourceUser is the logging subsystem for user resources.
	LogResourceUser = "resource-user"
	// LogResourceSecret is the logging subsystem for secret resources.
	LogResourceSecret = "resource-secret"
	// LogResourceAccessSecret is the logging subsystem for access secret resources.
	LogResourceAccessSecret = "resource-access-secret"
	// LogResourceStoragePool is the logging subsystem for storage pool resources.
	LogResourceStoragePool = "resource-storage-pool"
	// LogResourceController is the logging subsystem for controller resources.
	LogResourceController = "resource-controller"
	// LogResourceCloud is the logging subsystem for cloud resources.
	LogResourceCloud = "resource-cloud"

	// LogDataSourceJAASGroup is the logging subsystem for JAAS group data sources.
	LogDataSourceJAASGroup = "datasource-jaas-group"
	// LogDataSourceJAASRole is the logging subsystem for JAAS role data sources.
	LogDataSourceJAASRole = "datasource-jaas-role"

	// LogResourceJAASAccessModel is the logging subsystem for JAAS access model resources.
	LogResourceJAASAccessModel = "resource-jaas-access-model"
	// LogResourceJAASAccessCloud is the logging subsystem for JAAS access cloud resources.
	LogResourceJAASAccessCloud = "resource-jaas-access-cloud"
	// LogResourceJAASAccessGroup is the logging subsystem for JAAS access group resources.
	LogResourceJAASAccessGroup = "resource-jaas-access-group"
	// LogResourceJAASAccessRole is the logging subsystem for JAAS access role resources.
	LogResourceJAASAccessRole = "resource-jaas-access-role"
	// LogResourceJAASAccessOffer is the logging subsystem for JAAS access offer resources.
	LogResourceJAASAccessOffer = "resource-jaas-access-offer"
	// LogResourceJAASAccessController is the logging subsystem for JAAS access controller resources.
	LogResourceJAASAccessController = "resource-jaas-access-controller"
	// LogResourceJAASAccessSvcAcc is the logging subsystem for JAAS access service account resources.
	LogResourceJAASAccessSvcAcc = "resource-jaas-access-service-account"
	// LogResourceJAASGroup is the logging subsystem for JAAS group resources.
	LogResourceJAASGroup = "resource-jaas-group"
	// LogResourceJAASRole is the logging subsystem for JAAS role resources.
	LogResourceJAASRole = "resource-jaas-role"
)

// LogResourceIntegration is the logging subsystem for integration resources.
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
