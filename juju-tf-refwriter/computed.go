// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package main

import "github.com/hashicorp/hcl/v2/hclwrite"

// computedAttributes lists the Computed-only attributes (Computed, not
// Optional, not Required) for each resource kind. These attributes cannot be
// set in config, so the literal values that `terraform query` emits for them
// fail validation on apply and are pruned. Attributes that are both
// Computed and Optional (or Computed and Required) are intentionally omitted:
// the user may set those, so they are kept.
//
// Keep this in sync with the provider schemas in internal/provider.
var computedAttributes = map[resourceKind]map[string]struct{}{
	kindModel: {
		"uuid": {},
		"id":   {},
	},
	kindApplication: {
		"id":           {},
		"model_type":   {},
		"unit_numbers": {},
		"storage":      {},
	},
	kindMachine: {
		"id":          {},
		"machine_id":  {},
		"instance_id": {},
		"hostname":    {},
	},
	kindSecret: {
		"id":         {},
		"secret_id":  {},
		"secret_uri": {},
	},
	kindIntegration: {
		"id": {},
	},
	kindAccessModel: {
		"id": {},
	},
	kindAccessSecret: {
		"id": {},
	},
}

// pruneComputedAttributes removes the Computed-only attributes for the given
// resource kind from a block. Returns true if any attribute was removed.
// Unknown kinds are left untouched.
func pruneComputedAttributes(block *hclwrite.Block, kind resourceKind) bool {
	names, ok := computedAttributes[kind]
	if !ok {
		return false
	}
	removed := false
	for name := range names {
		if block.Body().GetAttribute(name) != nil {
			block.Body().RemoveAttribute(name)
			removed = true
		}
	}
	return removed
}
