// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/require"
)

type revisionModifierTestModel struct {
	Charm types.List `tfsdk:"charm"`
}

func TestInvalidateRevisionIfChannelOrBaseChanges(t *testing.T) {
	testCases := []struct {
		name           string
		stateChannel   string
		planChannel    types.String
		stateBase      types.String
		planBase       types.String
		configRevision types.Int64
		wantUnknown    bool
	}{
		{
			name:           "channel changes with unpinned revision",
			stateChannel:   "latest/stable",
			planChannel:    types.StringValue("latest/edge"),
			stateBase:      types.StringValue("ubuntu@22.04"),
			planBase:       types.StringValue("ubuntu@22.04"),
			configRevision: types.Int64Null(),
			wantUnknown:    true,
		},
		{
			name:           "base changes with unpinned revision",
			stateChannel:   "latest/stable",
			planChannel:    types.StringValue("latest/stable"),
			stateBase:      types.StringValue("ubuntu@22.04"),
			planBase:       types.StringValue("ubuntu@24.04"),
			configRevision: types.Int64Null(),
			wantUnknown:    true,
		},
		{
			name:           "base remains unchanged with unpinned revision",
			stateChannel:   "latest/stable",
			planChannel:    types.StringValue("latest/stable"),
			stateBase:      types.StringValue("ubuntu@22.04"),
			planBase:       types.StringValue("ubuntu@22.04"),
			configRevision: types.Int64Null(),
			wantUnknown:    false,
		},
		{
			name:           "base changes with pinned revision",
			stateChannel:   "latest/stable",
			planChannel:    types.StringValue("latest/stable"),
			stateBase:      types.StringValue("ubuntu@22.04"),
			planBase:       types.StringValue("ubuntu@24.04"),
			configRevision: types.Int64Value(123),
			wantUnknown:    false,
		},
		{
			name:           "unknown computed base does not invalidate revision",
			stateChannel:   "latest/stable",
			planChannel:    types.StringValue("latest/stable"),
			stateBase:      types.StringValue("ubuntu@22.04"),
			planBase:       types.StringUnknown(),
			configRevision: types.Int64Null(),
			wantUnknown:    false,
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			testSchema := revisionModifierTestSchema()
			state := revisionModifierTestState(t, ctx, testSchema, types.StringValue(tt.stateChannel), tt.stateBase)
			plan := revisionModifierTestState(t, ctx, testSchema, tt.planChannel, tt.planBase)
			response := planmodifier.Int64Response{PlanValue: types.Int64Value(100)}

			InvalidateRevisionIfChannelOrBaseChanges().PlanModifyInt64(ctx, planmodifier.Int64Request{
				Path:        path.Root("charm").AtListIndex(0).AtName("revision"),
				ConfigValue: tt.configRevision,
				Plan:        tfsdk.Plan{Raw: plan.Raw, Schema: testSchema},
				State:       state,
			}, &response)

			require.False(t, response.Diagnostics.HasError(), response.Diagnostics.Errors())
			require.Equal(t, tt.wantUnknown, response.PlanValue.IsUnknown())
		})
	}
}

func revisionModifierTestSchema() schema.Schema {
	return schema.Schema{
		Blocks: map[string]schema.Block{
			"charm": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"channel":  schema.StringAttribute{Optional: true},
						"base":     schema.StringAttribute{Optional: true},
						"revision": schema.Int64Attribute{Optional: true},
					},
				},
			},
		},
	}
}

func revisionModifierTestState(t *testing.T, ctx context.Context, testSchema schema.Schema, channel, base types.String) tfsdk.State {
	t.Helper()

	charmType := types.ObjectType{AttrTypes: map[string]attr.Type{
		"channel":  types.StringType,
		"base":     types.StringType,
		"revision": types.Int64Type,
	}}
	state := tfsdk.State{Schema: testSchema}
	diags := state.Set(ctx, revisionModifierTestModel{
		Charm: types.ListValueMust(charmType, []attr.Value{
			types.ObjectValueMust(charmType.AttrTypes, map[string]attr.Value{
				"channel":  channel,
				"base":     base,
				"revision": types.Int64Value(100),
			}),
		}),
	})
	require.False(t, diags.HasError(), diags.Errors())

	return state
}
