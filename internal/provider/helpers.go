// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// model names for logging
// @module=juju.<subsystem>
// e.g.:
//
//	@module=juju.resource-application
const LogDataSourceMachine = "datasource-machine"

func addClientNotConfiguredError(diag *diag.Diagnostics, resource, method string) {
	diag.AddError(
		"Provider Error, Client Not Configured",
		fmt.Sprintf("Unable to %s %s resource. Expected configured Juju Client. "+
			"Please report this issue to the provider developers.", method, resource),
	)
}
