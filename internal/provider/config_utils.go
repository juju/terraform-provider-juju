// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newConfig converts a types.Map (from state or plan) to a map[string]string.
// nil values in the types.Map are ignored.
func newConfig(ctx context.Context, configPlan types.Map) (map[string]string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	var config map[string]*string
	diags.Append(configPlan.ElementsAs(ctx, &config, false)...)
	if diags.HasError() {
		return nil, diags
	}
	configMap := map[string]string{}
	for k, v := range config {
		if v != nil {
			configMap[k] = *v
		}
	}
	return configMap, diags
}

// newConfigFromModelConfigAPI converts the config returned by the ModelConfig
// API to a map[string]*string, filtering out any keys that are not present
// in the configFromState types.Map. This ensures that only user-defined config
// keys are included, avoiding any default or system-defined keys that may be
// returned by the API.
func newConfigFromModelConfigAPI(ctx context.Context, configFromAPI map[string]interface{}, configFromState types.Map) (map[string]*string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	config := map[string]*string{}
	stateConfig := map[string]*string{}
	diags.Append(configFromState.ElementsAs(ctx, &stateConfig, false)...)
	if diags.HasError() {
		return nil, diags
	}
	// If there is no config in state, return nil to avoid returning
	// an empty map. We need that because in the state we have nil, and
	// if we return an empty map here, the state will see that as a change.
	if len(stateConfig) == 0 {
		return nil, nil
	}
	// Only include config keys that are present in the stateConfig.
	// The Juju API may return additional config keys that were not set by the user,
	// even when using the ModelGetWithMetadata facade. Some keys may have Source != "default"
	// but were not explicitly defined by the user. To avoid returning unwanted keys,
	// we filter configFromAPI to only those keys present in stateConfig.
	for k, v := range stateConfig {
		if v != nil {
			if value, exists := configFromAPI[k]; exists {
				stringifiedValue := fmt.Sprint(value)
				config[k] = &stringifiedValue
			}
		} else {
			config[k] = nil
		}
	}

	return config, nil
}

// computeConfigDiff compares the config in state and plan, and returns
// the new config map to set, and the list of config keys to unset.
// nil values in the plan that exist in the state are treated as keys to unset.
func computeConfigDiff(ctx context.Context, configState types.Map, configPlan types.Map) (map[string]string, []string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	stateConfig := map[string]*string{}
	diags.Append(configState.ElementsAs(ctx, &stateConfig, false)...)
	if diags.HasError() {
		return nil, nil, diags
	}
	planConfig := map[string]*string{}
	diags.Append(configPlan.ElementsAs(ctx, &planConfig, false)...)
	if diags.HasError() {
		return nil, nil, diags
	}
	unsetConfigKeys := []string{}
	for k, v := range stateConfig {
		if v != nil {
			if newV, ok := planConfig[k]; !ok || newV == nil {
				unsetConfigKeys = append(unsetConfigKeys, k)
			}
		}
	}
	newConfigMapNotNil := map[string]string{}
	for k, v := range planConfig {
		if v != nil {
			newConfigMapNotNil[k] = *v
		}
	}

	return newConfigMapNotNil, unsetConfigKeys, diags
}
