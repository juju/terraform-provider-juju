// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/terraform-provider-juju/internal/juju"
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

// newConfigFromMap converts a map[string]interface{} to a map[string]*string, ignoring nil values.
func newConfigFromMap(configMap map[string]interface{}) map[string]*string {
	config := map[string]*string{}
	for k, v := range configMap {
		if v != nil {
			config[k] = castToJujuConfig(v)
		} else {
			config[k] = nil
		}
	}
	return config
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
	if configFromState.IsNull() || configFromState.IsUnknown() {
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
				config[k] = castToJujuConfig(value)
			} else {
				config[k] = v
			}
		} else {
			config[k] = nil
		}
	}

	return config, nil
}

// castToJujuConfig converts interface{} values to
// the right string representation expected by Juju.
func castToJujuConfig(v interface{}) *string {
	if v == nil {
		return nil
	}
	switch v := v.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return nil
		}
		s := ""
		for mk, mv := range v {
			s += fmt.Sprintf("%s=%v", mk, mv)
		}
		return &s
	case []string:
		if len(v) == 0 {
			return nil
		}
		s := strings.Join(v, ",")
		return &s
	default:
		stringifiedValue := fmt.Sprint(v)
		return &stringifiedValue
	}
}

// newConfigFromApplicationAPI converts the config returned by the
// ReadgApplication API to a map[string]*string, filtering out any keys that
// are at their default value. And adding the keys in the state but not in
// the API response.
func newConfigFromApplicationAPI(_ context.Context, configFromAPI map[string]juju.ConfigEntry, configFromState types.Map) (map[string]*string, diag.Diagnostics) {
	diags := diag.Diagnostics{}
	config := map[string]*string{}
	stateConfig := map[string]*string{}
	diags.Append(configFromState.ElementsAs(context.Background(), &stateConfig, false)...)
	if diags.HasError() {
		return nil, diags
	}
	// Add all non-default config from the API.
	for k, v := range configFromAPI {
		if !v.IsDefault {
			stringifiedValue := v.String()
			config[k] = &stringifiedValue
		}
	}

	// Add all config from the state that is not in the API response.
	for k, v := range stateConfig {
		if _, exists := config[k]; !exists {
			config[k] = v
		}
	}
	// If the state config is nil or unknown, return nil to avoid returning
	// an empty map. We need that because in the state we have nil, and
	// if we return an empty map here, the state will see that as a change.
	if len(config) == 0 {
		if configFromState.IsNull() || configFromState.IsUnknown() {
			return nil, nil
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
