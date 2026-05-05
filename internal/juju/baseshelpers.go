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
