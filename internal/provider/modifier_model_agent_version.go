// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// AgentVersionCreateOnlyModifier blocks agent_version from being configured
// during model creation while still allowing updates on existing resources.
func AgentVersionCreateOnlyModifier() planmodifier.String {
	return agentVersionCreateOnlyModifier{}
}

type agentVersionCreateOnlyModifier struct{}

func (m agentVersionCreateOnlyModifier) Description(_ context.Context) string {
	return "Prevents agent_version from being configured when creating a model"
}

func (m agentVersionCreateOnlyModifier) MarkdownDescription(_ context.Context) string {
	return "Prevents `agent_version` from being configured when creating a model"
}

func (m agentVersionCreateOnlyModifier) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.State.Raw.IsNull() {
		return
	}

	if req.ConfigValue.IsNull() {
		return
	}

	resp.Diagnostics.AddAttributeError(
		req.Path,
		"Invalid agent_version for model creation",
		"agent_version cannot be set when creating a model. Juju creates new models at the controller version; set this attribute only after the model exists to request an upgrade.",
	)
}
