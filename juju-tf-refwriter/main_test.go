// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRewriteTransformation runs each input fixture in in/ through
// transformTerraformFile and compares the result to the matching file in out/.
func TestRewriteTransformation(t *testing.T) {
	inDir := "in"
	outDir := "out"

	inFiles, err := filepath.Glob(filepath.Join(inDir, "*.tf"))
	require.NoError(t, err, "Error reading input directory")
	require.NotEmpty(t, inFiles, "No .tf files found in %s", inDir)

	t.Logf("Testing %d files from %s against expected outputs in %s", len(inFiles), inDir, outDir)

	for _, inFile := range inFiles {
		t.Run(filepath.Base(inFile), func(t *testing.T) {
			filename := filepath.Base(inFile)
			outFile := filepath.Join(outDir, filename)

			require.FileExists(t, outFile, "Expected output file not found")

			inContent, err := os.ReadFile(inFile)
			require.NoError(t, err, "Error reading input file")

			expectedContent, err := os.ReadFile(outFile)
			require.NoError(t, err, "Error reading expected output file")

			result, err := transformTerraformFile(inContent, filename)
			require.NoError(t, err, "Error transforming file")

			if !bytes.Equal(result.ModifiedContent, expectedContent) {
				actualFile := filepath.Join("actual_" + filename)
				_ = os.WriteFile(actualFile, result.ModifiedContent, 0644)

				assert.Equal(t, string(expectedContent), string(result.ModifiedContent),
					"Transformation does not match expected output. Actual output saved to %s", actualFile)
			}
		})
	}
}

func TestDiscoverTerraformFiles(t *testing.T) {
	tests := []struct {
		name          string
		target        string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "discover files from in folder",
			target:        "in",
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "discover files from in folder with relative path",
			target:        filepath.Join(".", "in"),
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "single file",
			target:        "in/full_model.tf",
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "non-existent path",
			target:        "non-existent-folder",
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := discoverTerraformFiles(tt.target)

			if tt.expectError {
				assert.Error(t, err, "expected error but got none")
				return
			}

			require.NoError(t, err, "unexpected error")
			assert.Len(t, files, tt.expectedCount, "unexpected number of files")

			for _, file := range files {
				assert.True(t, len(file) >= 3, "file path too short: %s", file)
				assert.Equal(t, ".tf", file[len(file)-3:], "file %s is not a .tf file", file)
			}
		})
	}
}

func TestParseIdentity(t *testing.T) {
	tests := []struct {
		name    string
		kind    resourceKind
		id      string
		wantErr bool
		check   func(t *testing.T, e entityID)
	}{
		{
			name: "model uuid",
			kind: kindModel,
			id:   "c1cecf1e-fe66-4589-8585-e579edd6f58b",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "c1cecf1e-fe66-4589-8585-e579edd6f58b", e.modelUUID)
			},
		},
		{
			name: "application",
			kind: kindApplication,
			id:   "c1cecf1e-fe66-4589-8585-e579edd6f58b:dummy-sink",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "c1cecf1e-fe66-4589-8585-e579edd6f58b", e.modelUUID)
				assert.Equal(t, "dummy-sink", e.appName)
			},
		},
		{
			name: "machine",
			kind: kindMachine,
			id:   "c1cecf1e-fe66-4589-8585-e579edd6f58b:1:machine-1",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "c1cecf1e-fe66-4589-8585-e579edd6f58b", e.modelUUID)
				assert.Equal(t, "1", e.machineID)
				assert.Equal(t, "machine-1", e.machineName)
			},
		},
		{
			name: "secret",
			kind: kindSecret,
			id:   "c1cecf1e-fe66-4589-8585-e579edd6f58b:db-password",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "db-password", e.secretName)
			},
		},
		{
			name: "ssh_key",
			kind: kindSSHKey,
			id:   "sshkey:c1cecf1e-fe66-4589-8585-e579edd6f58b:abc123",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "c1cecf1e-fe66-4589-8585-e579edd6f58b", e.modelUUID)
				assert.Equal(t, "abc123", e.keyID)
			},
		},
		{
			name: "integration",
			kind: kindIntegration,
			id:   "c1cecf1e-fe66-4589-8585-e579edd6f58b:dummy-sink:sink:dummy-source:source",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "dummy-sink", e.app1)
				assert.Equal(t, "sink", e.ep1)
				assert.Equal(t, "dummy-source", e.app2)
				assert.Equal(t, "source", e.ep2)
			},
		},
		{
			name: "offer url",
			kind: kindOffer,
			id:   "mycontroller:test4.dummy-sink",
			check: func(t *testing.T, e entityID) {
				assert.Equal(t, "mycontroller:test4.dummy-sink", e.offerURL)
			},
		},
		{
			name:    "malformed application",
			kind:    kindApplication,
			id:      "no-colon",
			wantErr: true,
		},
		{
			name:    "malformed machine",
			kind:    kindMachine,
			id:      "uuid:1",
			wantErr: true,
		},
		{
			name:    "malformed ssh_key missing prefix",
			kind:    kindSSHKey,
			id:      "uuid:abc",
			wantErr: true,
		},
		{
			name:    "malformed integration",
			kind:    kindIntegration,
			id:      "uuid:app:ep:app",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e, err := parseIdentity(tt.kind, tt.id)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			if tt.check != nil {
				tt.check(t, e)
			}
		})
	}
}

func TestKindOf(t *testing.T) {
	tests := []struct {
		input string
		want  resourceKind
	}{
		{"juju_model", kindModel},
		{"juju_application", kindApplication},
		{"juju_machine", kindMachine},
		{"juju_secret", kindSecret},
		{"juju_space", kindSpace},
		{"juju_ssh_key", kindSSHKey},
		{"juju_storage_pool", kindStoragePool},
		{"juju_offer", kindOffer},
		{"juju_integration", kindIntegration},
		{"juju_access_model", kindAccessModel},
		{"juju_access_secret", kindAccessSecret},
		{"juju_unknown", kindUnknown},
		{"", kindUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, kindOf(tt.input))
		})
	}
}

// TestRewriteModelUUIDOnly verifies that a resource with only a model_uuid
// (no machines, no integration apps) gets its model_uuid rewritten and nothing
// else changes.
func TestRewriteModelUUIDOnly(t *testing.T) {
	src := []byte(`
resource "juju_model" "m" {
  uuid = "uuid-a"
}

import {
  to       = juju_model.m
  provider = juju
  identity = {
    id = "uuid-a"
  }
}

resource "juju_secret" "s" {
  model_uuid = "uuid-a"
  name       = "pw"
}

import {
  to       = juju_secret.s
  provider = juju
  identity = {
    id = "uuid-a:pw"
  }
}
`)
	result, err := transformTerraformFile(src, "test.tf")
	require.NoError(t, err)
	assert.Contains(t, string(result.ModifiedContent), "model_uuid = juju_model.m.uuid")
	assert.Equal(t, 0, result.Warnings)
}

// TestRewriteMissingModelWarns verifies that when no juju_model resource
// matches, the literal is left in place and a warning is recorded.
func TestRewriteMissingModelWarns(t *testing.T) {
	src := []byte(`
resource "juju_secret" "s" {
  model_uuid = "uuid-a"
  name       = "pw"
}

import {
  to       = juju_secret.s
  provider = juju
  identity = {
    id = "uuid-a:pw"
  }
}
`)
	result, err := transformTerraformFile(src, "test.tf")
	require.NoError(t, err)
	assert.Contains(t, string(result.ModifiedContent), `model_uuid = "uuid-a"`)
	assert.Equal(t, 1, result.Warnings)
}

// TestAlreadyReferencedModelUUIDIsLeftAlone verifies that a model_uuid that is
// already a reference (not a literal) is not touched.
func TestAlreadyReferencedModelUUIDIsLeftAlone(t *testing.T) {
	src := []byte(`
resource "juju_model" "m" {
  uuid = "uuid-a"
}

import {
  to       = juju_model.m
  provider = juju
  identity = {
    id = "uuid-a"
  }
}

resource "juju_secret" "s" {
  model_uuid = juju_model.m.uuid
  name       = "pw"
}

import {
  to       = juju_secret.s
  provider = juju
  identity = {
    id = "uuid-a:pw"
  }
}
`)
	result, err := transformTerraformFile(src, "test.tf")
	require.NoError(t, err)
	assert.Contains(t, string(result.ModifiedContent), "model_uuid = juju_model.m.uuid")
	// The model's computed `uuid` is still pruned, so the file is
	// rewritten even though the already-referenced model_uuid is left alone.
	assert.True(t, result.WasRewritten, "the computed uuid should be pruned")
	assert.NotContains(t, string(result.ModifiedContent), `uuid = "uuid-a"`)
}

// TestPruneComputedAttributes verifies that Computed-only attributes are
// removed for each resource kind, while Optional attributes (even Computed
// ones like `name`) are kept.
func TestPruneComputedAttributes(t *testing.T) {
	src := []byte(`
resource "juju_model" "m" {
  name = "test"
  uuid = "uuid-a"
}

import {
  to       = juju_model.m
  provider = juju
  identity = {
    id = "uuid-a"
  }
}

resource "juju_machine" "mach" {
  model_uuid = juju_model.m.uuid
  name       = "machine-1"
  machine_id = "1"
}

import {
  to       = juju_machine.mach
  provider = juju
  identity = {
    id = "uuid-a:1:machine-1"
  }
}
`)
	result, err := transformTerraformFile(src, "test.tf")
	require.NoError(t, err)
	out := string(result.ModifiedContent)
	// Computed-only attributes are pruned.
	assert.NotContains(t, out, `uuid = "uuid-a"`, "model uuid should be pruned")
	assert.NotContains(t, out, `machine_id = "1"`, "machine_id should be pruned")
	// Optional attributes are kept.
	assert.Contains(t, out, `name = "test"`, "model name should be kept")
	assert.Contains(t, out, `name       = "machine-1"`, "machine name should be kept")
}

// TestPruneNullAttributes verifies that attributes set to `null` are removed,
// while non-null values (including `0`, `false`, and `{}`) are kept.
func TestPruneNullAttributes(t *testing.T) {
	src := []byte(`
resource "juju_application" "a" {
  model_uuid         = "uuid-a"
  name               = "dummy-sink"
  config             = null
  endpoint_bindings  = null
  storage_directives = {}
  trust              = false
}

import {
  to       = juju_application.a
  provider = juju
  identity = {
    id = "uuid-a:dummy-sink"
  }
}
`)
	result, err := transformTerraformFile(src, "test.tf")
	require.NoError(t, err)
	out := string(result.ModifiedContent)
	assert.NotContains(t, out, "config = null", "null config should be pruned")
	assert.NotContains(t, out, "endpoint_bindings = null", "null endpoint_bindings should be pruned")
	// `false` and `{}` are not `null` and must be kept. The formatter
	// re-aligns attributes, so check the values rather than exact spacing.
	assert.Contains(t, out, "false", "false should be kept")
	assert.Contains(t, out, "{}", "empty map should be kept")
	assert.Contains(t, out, "storage_directives", "storage_directives should be kept")
	assert.Contains(t, out, "trust", "trust should be kept")
}
