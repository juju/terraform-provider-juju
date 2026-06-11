// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testing

import goTesting "testing"

func TestCompareVersions(t *goTesting.T) {
	testCases := []struct {
		name     string
		version1 string
		version2 string
		expected int
	}{
		{
			name:     "equal stable versions",
			version1: "4.0.12",
			version2: "4.0.12",
			expected: 0,
		},
		{
			name:     "stable less than stable",
			version1: "4.0.12",
			version2: "4.1.0",
			expected: -1,
		},
		{
			name:     "prerelease less than stable",
			version1: "4.1-beta1",
			version2: "4.1.0",
			expected: -1,
		},
		{
			name:     "stable greater than prerelease",
			version1: "4.1.0",
			version2: "4.1-beta1",
			expected: 1,
		},
		{
			name:     "later prerelease greater than earlier stable minor",
			version1: "4.1-beta1",
			version2: "4.0.12",
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *goTesting.T) {
			result := CompareVersions(tc.version1, tc.version2)
			if result != tc.expected {
				t.Fatalf("expected %d, got %d", tc.expected, result)
			}
		})
	}
}

func TestCompareVersionsPanicsOnInvalidVersion(t *goTesting.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for invalid version")
		}
	}()

	CompareVersions("4.1.0-beta1", "4.1.0")
}
