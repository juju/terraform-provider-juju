// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPlacementRequiresReplacefunc verifies that a change to the placement
// directive only forces replacement when the prior state already tracks a
// placement value. An imported machine has a null placement in state (Juju
// does not expose the directive), so it must not be replaced - see issue #1149.
func TestPlacementRequiresReplacefunc(t *testing.T) {
	tests := []struct {
		name        string
		stateValue  types.String
		configValue types.String
		planValue   types.String
		wantReplace bool
	}{
		{
			name:        "imported machine (null state) gaining a placement is not replaced",
			stateValue:  types.StringNull(),
			configValue: types.StringValue("zone=ThinkPad-X1"),
			planValue:   types.StringValue("zone=ThinkPad-X1"),
			wantReplace: false,
		},
		{
			name:        "placement removed from configuration is not replaced",
			stateValue:  types.StringValue("zone=ThinkPad-X1"),
			configValue: types.StringNull(),
			planValue:   types.StringNull(),
			wantReplace: false,
		},
		{
			name:        "placement changed on a tracked machine forces replacement",
			stateValue:  types.StringValue("zone=old"),
			configValue: types.StringValue("zone=new"),
			planValue:   types.StringValue("zone=new"),
			wantReplace: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := planmodifier.StringRequest{
				StateValue:  tt.stateValue,
				ConfigValue: tt.configValue,
				PlanValue:   tt.planValue,
			}
			resp := &stringplanmodifier.RequiresReplaceIfFuncResponse{}
			placementRequiresReplacefunc(context.Background(), req, resp)
			if resp.RequiresReplace != tt.wantReplace {
				t.Fatalf("RequiresReplace = %v, want %v", resp.RequiresReplace, tt.wantReplace)
			}
		})
	}
}
