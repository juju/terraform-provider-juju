// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

// disallowUnsetIfSet is a plan modifier which prevents an optional string
// attribute from being unset once it has been set.
//
// This is useful for Juju fields which cannot reliably be cleared once stored
// on the controller. Without this, removing the attribute from configuration
// would cause perpetual drift (Juju keeps returning the old value).
//
// Semantics:
//   - If prior state is null/unknown: allow any planned value.
//   - If prior state is non-null:
//   - plan null => error
//   - plan non-null/unknown => allow
type disallowUnsetIfSet struct {
	reason string
}

// DisallowUnsetIfSet returns a plan modifier that blocks transitions from
// non-null state to null plan.
func DisallowUnsetIfSet(reason string) planmodifier.String {
	return disallowUnsetIfSet{reason: reason}
}

func (m disallowUnsetIfSet) Description(_ context.Context) string {
	return "prevents an attribute from being unset once set"
}

func (m disallowUnsetIfSet) MarkdownDescription(_ context.Context) string {
	return "Prevents an attribute from being unset once set."
}

func (m disallowUnsetIfSet) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	// Can't enforce if we don't know what was set before.
	if req.StateValue.IsUnknown() {
		return
	}
	// Never set before so unsetting is fine.
	if req.StateValue.IsNull() {
		return
	}
	// If new value is unknown, don't block planning.
	if req.PlanValue.IsUnknown() {
		return
	}

	// Was set before, and plan is trying to unset.
	if req.PlanValue.IsNull() {
		detail := m.reason
		if detail == "" {
			detail = "this field cannot be unset once configured"
		}
		resp.Diagnostics.Append(diag.NewAttributeErrorDiagnostic(
			req.Path,
			"Unsupported change",
			fmt.Sprintf("%s (workaround for a Juju limitation)", detail),
		))
	}
}
