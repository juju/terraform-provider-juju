// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTerraformTransformation(t *testing.T) {
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

			// Check if expected output file exists
			require.FileExists(t, outFile, "Expected output file not found")

			// Read input file
			inContent, err := os.ReadFile(inFile)
			require.NoError(t, err, "Error reading input file")

			// Read expected output file
			expectedContent, err := os.ReadFile(outFile)
			require.NoError(t, err, "Error reading expected output file")

			// Transform the input
			result, err := transformTerraformFile(inContent, filename)
			require.NoError(t, err, "Error transforming file")

			// Compare the result with expected output
			if !bytes.Equal(result.ModifiedContent, expectedContent) {
				// Save actual output for debugging
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
			expectedCount: 18,
			expectError:   false,
		},
		{
			name:          "discover files from in folder with relative path",
			target:        filepath.Join(".", "in"),
			expectedCount: 18,
			expectError:   false,
		},
		{
			name:          "single file",
			target:        "in/juju_application_test.tf",
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

			// Verify all returned files end with .tf
			for _, file := range files {
				assert.True(t, len(file) >= 3, "file path too short: %s", file)
				assert.Equal(t, ".tf", file[len(file)-3:], "file %s is not a .tf file", file)
			}

			// Verify no "_upgraded" files are included
			for _, file := range files {
				assert.NotContains(t, file, "_upgraded", "found file with '_upgraded' in results: %s", file)
			}
		})
	}
}

func TestTerraformZeroVersionRegex(t *testing.T) {
	re := regexp.MustCompile(version0Regex)

	tests := []struct {
		input   string
		matches bool
	}{
		{`version = "0.1.2"`, true},
		{`version = "~>0.1.2"`, true},
		{`version = ">=0.1.2"`, true},
		{`version = "<0.1.2"`, true},
		{`version = "!=0.1.2"`, true},
		{`version = "<=0.1.2"`, true},
		{`version = ">0.1.2"`, true},
		{`version = ">0.1.2-beta"`, true},
		{`version = "1.0.0"`, false},
		{`version = ">=1.0.0"`, false},
		{`version = "1.0.0-beta3"`, false},
		{`version = "2.3.4"`, false},
	}

	for _, test := range tests {
		if got := re.MatchString(test.input); got != test.matches {
			t.Errorf("input: %q, expected match: %v, got: %v", test.input, test.matches, got)
		}
	}
}
