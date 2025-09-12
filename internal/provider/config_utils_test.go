// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func stringP(s string) *string {
	return &s
}

func TestNewConfig(t *testing.T) {
	mapToTest := map[string]*string{
		"key1": nil,
		"key2": stringP("value2"),
		"key3": stringP("value3"),
		"key4": nil,
		"key5": stringP(""),
	}
	tfMapToTest, diags := types.MapValueFrom(t.Context(), types.StringType, mapToTest)
	require.False(t, diags.HasError(), "failed to create types.Map from map: %v", diags)

	config, diags := newConfig(t.Context(), tfMapToTest)
	require.False(t, diags.HasError(), "NewConfig returned diagnostics: %v", diags)

	expectedConfig := map[string]string{
		"key2": "value2",
		"key3": "value3",
		"key5": "",
	}
	assert.Equal(t, config, expectedConfig, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestNewConfigFromModelConfigAPI(t *testing.T) {
	mapFromAPI := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}
	mapFromState := map[string]*string{
		"key2": stringP("value2"),
		"key3": nil,
		"key5": stringP("value5"),
	}
	tfMapFromState, diags := types.MapValueFrom(t.Context(), types.StringType, mapFromState)
	require.False(t, diags.HasError(), "failed to create types.Map from map: %v", diags)

	config, diags := newConfigFromModelConfigAPI(t.Context(), mapFromAPI, tfMapFromState)
	require.False(t, diags.HasError(), "NewConfigFromModelConfigAPI returned diagnostics: %v", diags)

	expectedConfig := map[string]*string{
		"key2": stringP("value2"),
		"key3": nil,
		"key5": stringP("value5"),
	}
	assert.Equal(t, config, expectedConfig, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestComputeConfigDiff(t *testing.T) {
	mapInState := map[string]*string{
		"key1": stringP("value1"),
		"key2": stringP("value2"),
		"key3": stringP("value3"),
		"key5": stringP("value5"),
		"key6": nil,
	}
	mapInPlan := map[string]*string{
		"key2": stringP("newValue2"), // updated
		"key3": nil,                  // to be unset
		"key4": stringP("value4"),    // new key
		"key5": stringP("value5"),    // unchanged
	}
	tfMapInState, diags := types.MapValueFrom(t.Context(), types.StringType, mapInState)
	require.False(t, diags.HasError(), "failed to create types.Map from mapInState: %v", diags)
	tfMapInPlan, diags := types.MapValueFrom(t.Context(), types.StringType, mapInPlan)
	require.False(t, diags.HasError(), "failed to create types.Map from mapInPlan: %v", diags)

	newConfig, keysToUnset, diags := computeConfigDiff(t.Context(), tfMapInState, tfMapInPlan)
	require.False(t, diags.HasError(), "ComputeConfigDiff returned diagnostics: %v", diags)

	expectedNewConfig := map[string]string{
		"key2": "newValue2",
		"key4": "value4",
		"key5": "value5",
	}
	assert.Equal(t, expectedNewConfig, newConfig, fmt.Sprintf("newConfig mismatch: got %+v, want %+v", newConfig, expectedNewConfig))

	expectedKeysToUnset := []string{"key3", "key1"}
	assert.Equal(t, len(expectedKeysToUnset), len(keysToUnset), "keysToUnset length mismatch")
	slices.Sort(keysToUnset)
	slices.Sort(expectedKeysToUnset)
	assert.Equal(t, expectedKeysToUnset, keysToUnset, "keysToUnset mismatch")
}

func TestNewConfigFromReaApplicationAPI(t *testing.T) {
	// Prepare configFromAPI with some default and non-default values
	configFromAPI := map[string]juju.ConfigEntry{
		"key1": {
			Value: "value1", IsDefault: false, // not default
		},
		"key2": {
			Value: "value2", IsDefault: true, // default, should be filtered out
		},
		"key3": {
			Value: "value3", IsDefault: false, // not default
		},
		"key6": {
			Value: "value6", IsDefault: true, // default, should be filtered out
		},
	}

	// Prepare stateConfig with some keys not present in configFromAPI
	stateConfigMap := map[string]*string{
		"key4": stringP("stateValue4"), // should be added since not in API
		"key5": nil,                    // should be added as nil
		"key6": nil,                    // should be added as nil, even if default in API
	}
	tfStateConfig, diags := types.MapValueFrom(t.Context(), types.StringType, stateConfigMap)
	require.False(t, diags.HasError(), "failed to create types.Map from stateConfigMap: %v", diags)

	config, diags := newConfigFromApplicationAPI(t.Context(), configFromAPI, tfStateConfig)
	require.False(t, diags.HasError(), "newConfigFromReaApplicationAPI returned diagnostics: %v", diags)

	expectedConfig := map[string]*string{
		"key1": stringP("value1"),      // from API, not default
		"key3": stringP("value3"),      // from API, not default
		"key4": stringP("stateValue4"), // from state, not in API
		"key5": nil,                    // from state, not in API
		"key6": nil,                    // from state, even if default in API
	}
	assert.Equal(t, expectedConfig, config, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestNewConfigFromReadApplicationAPINilState(t *testing.T) {
	var configFromAPI map[string]juju.ConfigEntry
	tfStateConfig, diags := types.MapValueFrom(t.Context(), types.StringType, configFromAPI)
	require.False(t, diags.HasError(), "failed to create types.Map from empty stateConfig: %v", diags)

	config, diags := newConfigFromApplicationAPI(t.Context(), configFromAPI, tfStateConfig)
	require.False(t, diags.HasError(), "newConfigFromReaApplicationAPI returned diagnostics: %v", diags)

	var expectedConfig map[string]*string // nil
	assert.Equal(t, expectedConfig, config, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestNewConfigFromReadApplicationAPIEmptyState(t *testing.T) {
	configFromAPI := map[string]juju.ConfigEntry{}

	tfStateConfig, diags := types.MapValueFrom(t.Context(), types.StringType, map[string]*string{})
	require.False(t, diags.HasError(), "failed to create types.Map from empty stateConfig: %v", diags)

	config, diags := newConfigFromApplicationAPI(t.Context(), configFromAPI, tfStateConfig)
	require.False(t, diags.HasError(), "newConfigFromReaApplicationAPI returned diagnostics: %v", diags)

	expectedConfig := map[string]*string{} // expect empty map
	assert.Equal(t, expectedConfig, config, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestNewConfigModelConfigAPINilState(t *testing.T) {
	var configFromAPI map[string]interface{}
	tfStateConfig, diags := types.MapValueFrom(t.Context(), types.StringType, configFromAPI)
	require.False(t, diags.HasError(), "failed to create types.Map from empty stateConfig: %v", diags)

	config, diags := newConfigFromModelConfigAPI(t.Context(), configFromAPI, tfStateConfig)
	require.False(t, diags.HasError(), "newConfigFromReaApplicationAPI returned diagnostics: %v", diags)

	var expectedConfig map[string]*string // nil
	assert.Equal(t, expectedConfig, config, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}

func TestNewConfigFromModelConfigAPIEmptyState(t *testing.T) {
	configFromAPI := map[string]interface{}{}

	tfStateConfig, diags := types.MapValueFrom(t.Context(), types.StringType, map[string]*string{})
	require.False(t, diags.HasError(), "failed to create types.Map from empty stateConfig: %v", diags)

	config, diags := newConfigFromModelConfigAPI(t.Context(), configFromAPI, tfStateConfig)
	require.False(t, diags.HasError(), "newConfigFromReaApplicationAPI returned diagnostics: %v", diags)

	expectedConfig := map[string]*string{} // expect empty map
	assert.Equal(t, expectedConfig, config, fmt.Sprintf("config mismatch: got %+v, want %+v", config, expectedConfig))
}
