// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testsing

import (
	"strconv"
	"strings"
)

// CompareVersions compares two versions in the format "X.Y.Z" and returns:
// -1 if version1 is less than version2
// 0 if version1 is equal to version2
// 1 if version1 is greater than version2
func CompareVersions(version1, version2 string) int {
	v1Parts := strings.Split(version1, ".")
	v2Parts := strings.Split(version2, ".")

	for i := 0; i < 3; i++ {
		v1, err := strconv.Atoi(v1Parts[i])
		if err != nil {
			panic(err)
		}
		v2, err := strconv.Atoi(v2Parts[i])
		if err != nil {
			panic(err)
		}

		if v1 < v2 {
			return -1
		} else if v1 > v2 {
			return 1
		}
	}

	return 0
}
