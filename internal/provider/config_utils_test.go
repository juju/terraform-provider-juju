// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider_test

import (
	"slices"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/juju/terraform-provider-juju/internal/provider"
)

func pointerToString(s string) *string {
	return &s
}

func TestNewConfig(t *testing.T) {
	mapToTest := map[string]*string{
		"key1": nil,
		"key2": pointerToString("value2"),
		"key3": pointerToString("value3"),
		"key4": nil,
	}
	tfMapToTest, diags := types.MapValueFrom(t.Context(), types.StringType, mapToTest)
	if diags.HasError() {
		t.Fatalf("failed to create types.Map from map: %v", diags)
	}
	config, diags := provider.NewConfig(t.Context(), tfMapToTest)
	if diags.HasError() {
		t.Fatalf("NewConfig returned diagnostics: %v", diags)
	}

	expectedConfig := map[string]string{
		"key2": "value2",
		"key3": "value3",
	}
	if len(config) != len(expectedConfig) {
		t.Fatalf("expected config length %d, got %d", len(expectedConfig), len(config))
	}
	for k, v := range expectedConfig {
		if config[k] != v {
			t.Errorf("expected config[%q] = %q, got %q", k, v, config[k])
		}
	}
}

func TestNewConfigFromModelConfigAPI(t *testing.T) {
	mapFromAPI := map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
		"key4": "value4",
	}
	mapFromState := map[string]*string{
		"key2": pointerToString("value2"),
		"key3": nil,
		"key5": pointerToString("value5"),
	}
	tfMapFromState, diags := types.MapValueFrom(t.Context(), types.StringType, mapFromState)
	if diags.HasError() {
		t.Fatalf("failed to create types.Map from map: %v", diags)
	}
	config, diags := provider.NewConfigFromModelConfigAPI(t.Context(), mapFromAPI, tfMapFromState)
	if diags.HasError() {
		t.Fatalf("NewConfigFromModelConfigAPI returned diagnostics: %v", diags)
	}

	expectedConfig := map[string]*string{
		"key2": pointerToString("value2"),
		"key5": nil,
	}
	if len(config) != len(expectedConfig) {
		t.Fatalf("expected config length %d, got %d", len(expectedConfig), len(config))
	}
	for k, v := range expectedConfig {
		if (v == nil && config[k] != nil) || (v != nil && config[k] == nil) {
			t.Errorf("expected config[%q] = %v, got %v", k, v, config[k])
		} else if v != nil && *v != *config[k] {
			t.Errorf("expected config[%q] = %q, got %q", k, *v, *config[k])
		}
	}
}

func TestComputeConfigDiff(t *testing.T) {
	mapInState := map[string]*string{
		"key1": pointerToString("value1"),
		"key2": pointerToString("value2"),
		"key3": pointerToString("value3"),
		"key5": pointerToString("value5"),
	}
	mapInPlan := map[string]*string{
		"key2": pointerToString("newValue2"), // updated
		"key3": nil,                          // to be unset
		"key4": pointerToString("value4"),    // new key
		"key5": pointerToString("value5"),    // unchanged
	}
	tfMapInState, diags := types.MapValueFrom(t.Context(), types.StringType, mapInState)
	if diags.HasError() {
		t.Fatalf("failed to create types.Map from mapInState: %v", diags)
	}
	tfMapInPlan, diags := types.MapValueFrom(t.Context(), types.StringType, mapInPlan)
	if diags.HasError() {
		t.Fatalf("failed to create types.Map from mapInPlan: %v", diags)
	}
	newConfig, keysToUnset, diags := provider.ComputeConfigDiff(t.Context(), tfMapInState, tfMapInPlan)
	if diags.HasError() {
		t.Fatalf("ComputeConfigDiff returned diagnostics: %v", diags)
	}

	expectedNewConfig := map[string]string{
		"key2": "newValue2",
		"key4": "value4",
		"key5": "value5",
	}

	if len(newConfig) != len(expectedNewConfig) {
		t.Fatalf("expected newConfig length %d, got %d", len(expectedNewConfig), len(newConfig))
	}
	for k, v := range expectedNewConfig {
		if newConfig[k] != v {
			t.Errorf("expected newConfig[%q] = %q, got %q", k, v, newConfig[k])
		}
	}

	expectedKeysToUnset := []string{"key3", "key1"}
	if len(keysToUnset) != len(expectedKeysToUnset) {
		t.Fatalf("expected keysToUnset length %d, got %d", len(expectedKeysToUnset), len(keysToUnset))
	}
	slices.Sort(keysToUnset)
	slices.Sort(expectedKeysToUnset)
	for i, k := range expectedKeysToUnset {
		if keysToUnset[i] != k {
			t.Errorf("expected keysToUnset[%d] = %q, got %q", i, k, keysToUnset[i])
		}
	}
}
