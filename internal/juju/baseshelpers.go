// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import "github.com/juju/juju/core/base"

// basesContain returns true if the provide base is contained
// in the provided slice of bases.
func basesContain(lookFor base.Base, slice []base.Base) bool {
	if lookFor.Empty() {
		return false
	}
	for _, v := range slice {
		if lookFor.IsCompatible(v) {
			return true
		}
	}
	return false
}

// intersectionOfBases returns a slice of bases which are included
// in each of two supplied.
func intersectionOfBases(one, two []base.Base) []base.Base {
	if len(one) == 0 && len(two) == 0 {
		return []base.Base{}
	}
	result := make([]base.Base, 0)
	for _, value1 := range one {
		for _, value2 := range two {
			if value1.IsCompatible(value2) {
				result = append(result, value1)
			}
		}
	}
	return result
}
