// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testing

import (
	version "github.com/juju/version/v2"
)

// CompareVersions compares two Juju versions and returns:
// -1 if version1 is less than version2
// 0 if version1 is equal to version2
// 1 if version1 is greater than version2
func CompareVersions(version1, version2 string) int {
	v1, err := version.Parse(version1)
	if err != nil {
		panic(err)
	}

	v2, err := version.Parse(version2)
	if err != nil {
		panic(err)
	}

	return v1.Compare(v2)
}
