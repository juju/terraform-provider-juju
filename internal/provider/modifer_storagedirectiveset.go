// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	jujustorage "github.com/juju/juju/storage"
)

// storageDirectivesMapRequiresReplace is a plan modifier function that
// determines if the storage directive map requires the application to be
// replaced. It compares the storage directive map in the plan with the
// storage directive map in state.
// Return false if new items were added and old items were not changed.
// Return true if old items were changed or removed.
func storageDirectivesMapRequiresReplace(ctx context.Context, req planmodifier.MapRequest, resp *mapplanmodifier.RequiresReplaceIfFuncResponse) {
	planSet := make(map[string]jujustorage.Constraints)
	if !req.PlanValue.IsUnknown() {
		var planStorageDirectives map[string]string
		resp.Diagnostics.Append(req.PlanValue.ElementsAs(ctx, &planStorageDirectives, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(planStorageDirectives) > 0 {
			for label, storage := range planStorageDirectives {
				cons, err := jujustorage.ParseConstraints(storage)
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse storage directives, got error: %s", err))
					continue
				}
				planSet[label] = cons
			}
		}
	}
	if resp.Diagnostics.HasError() {
		return
	}
	stateSet := make(map[string]jujustorage.Constraints)
	if !req.StateValue.IsUnknown() {
		var stateStorageDirectives map[string]string
		resp.Diagnostics.Append(req.StateValue.ElementsAs(ctx, &stateStorageDirectives, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if len(stateStorageDirectives) > 0 {
			for label, storage := range stateStorageDirectives {
				cons, err := jujustorage.ParseConstraints(storage)
				if err != nil {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse storage directives, got error: %s", err))
					continue
				}
				stateSet[label] = cons
			}
		}
	}

	// Return false if new items were added and old items were not changed
	for key, value := range planSet {
		stateValue, ok := stateSet[key]
		if !ok {
			resp.RequiresReplace = false
			return
		}
		if (value.Size != stateValue.Size) || (value.Pool != stateValue.Pool) || (value.Count != stateValue.Count) {
			resp.RequiresReplace = true
			return
		}
	}

	// Return true if old items were removed
	for key := range stateSet {
		if _, ok := planSet[key]; !ok {
			resp.RequiresReplace = true
			return
		}
	}

	resp.RequiresReplace = false
}
