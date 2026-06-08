// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.DataSourceWithConfigure = &subnetsDataSource{}

const (
	subnetAttrCIDR              = "cidr"
	subnetAttrProviderID        = "provider_id"
	subnetAttrProviderNetworkID = "provider_network_id"
	subnetAttrProviderSpaceID   = "provider_space_id"
	subnetAttrVLANTag           = "vlan_tag"
	subnetAttrLife              = "life"
	subnetAttrSpaceName         = "space_name"
	subnetAttrZones             = "zones"
)

var subnetObjectAttrTypes = map[string]attr.Type{
	subnetAttrCIDR:              types.StringType,
	subnetAttrProviderID:        types.StringType,
	subnetAttrProviderNetworkID: types.StringType,
	subnetAttrProviderSpaceID:   types.StringType,
	subnetAttrVLANTag:           types.Int64Type,
	subnetAttrLife:              types.StringType,
	subnetAttrSpaceName:         types.StringType,
	subnetAttrZones:             types.ListType{ElemType: types.StringType},
}

var subnetNestedSchemaAttributes = map[string]schema.Attribute{
	subnetAttrCIDR:              schema.StringAttribute{Computed: true},
	subnetAttrProviderID:        schema.StringAttribute{Computed: true},
	subnetAttrProviderNetworkID: schema.StringAttribute{Computed: true},
	subnetAttrProviderSpaceID:   schema.StringAttribute{Computed: true},
	subnetAttrVLANTag:           schema.Int64Attribute{Computed: true},
	subnetAttrLife:              schema.StringAttribute{Computed: true},
	subnetAttrSpaceName:         schema.StringAttribute{Computed: true},
	subnetAttrZones: schema.ListAttribute{
		Computed:    true,
		ElementType: types.StringType,
	},
}

// NewSubnetsDataSource returns a subnets data source.
func NewSubnetsDataSource() datasource.DataSource {
	return &subnetsDataSource{}
}

type subnetsDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type subnetsDataSourceModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	SpaceName types.String `tfsdk:"space_name"`
	ZoneName  types.String `tfsdk:"zone_name"`
	Subnets   types.Map    `tfsdk:"subnets"`

	// ID required by the testing framework.
	ID types.String `tfsdk:"id"`
}

func (d *subnetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnets"
}

func (d *subnetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a list of Juju subnets.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where subnets exist.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"space_name": schema.StringAttribute{
				Description: "Optional space name filter.",
				Optional:    true,
			},
			"zone_name": schema.StringAttribute{
				Description: "Optional availability zone filter.",
				Optional:    true,
			},
			"subnets": schema.MapNestedAttribute{
				Description: "Map of subnets keyed by CIDR.",
				Computed:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: subnetNestedSchemaAttributes,
				},
			},
			"id": schema.StringAttribute{
				Description: "Identifier of the subnets data source.",
				Computed:    true,
			},
		},
	}
}

func (d *subnetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderDataForDataSource(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceSubnets)
}

func (d *subnetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "subnets")
		return
	}

	var data subnetsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := juju.ListSubnetsInput{
		ModelUUID: data.ModelUUID.ValueString(),
		SpaceName: data.SpaceName.ValueString(),
		Zone:      data.ZoneName.ValueString(),
	}
	subnets, err := d.client.Subnets.ListSubnets(ctx, &input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read subnets data source, got error: %s", err))
		return
	}
	d.trace("read subnets data source", map[string]any{
		"model_uuid": input.ModelUUID,
		"space_name": input.SpaceName,
		"zone_name":  input.Zone,
		"count":      len(subnets),
	})

	subnetObjectType := types.ObjectType{AttrTypes: subnetObjectAttrTypes}

	result := make(map[string]attr.Value, len(subnets))
	for _, subnet := range subnets {
		zones, diags := types.ListValueFrom(ctx, types.StringType, subnet.Zones)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		obj, diags := types.ObjectValue(subnetObjectType.AttrTypes, map[string]attr.Value{
			subnetAttrCIDR:              types.StringValue(subnet.CIDR),
			subnetAttrProviderID:        types.StringValue(subnet.ProviderID),
			subnetAttrProviderNetworkID: types.StringValue(subnet.ProviderNetworkID),
			subnetAttrProviderSpaceID:   types.StringValue(subnet.ProviderSpaceID),
			subnetAttrVLANTag:           types.Int64Value(int64(subnet.VLANTag)),
			subnetAttrLife:              types.StringValue(string(subnet.Life)),
			subnetAttrSpaceName:         types.StringValue(subnet.SpaceName),
			subnetAttrZones:             zones,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		result[subnet.CIDR] = obj
	}

	subnetsMap, diags := types.MapValue(subnetObjectType, result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Subnets = subnetsMap
	data.ID = types.StringValue(fmt.Sprintf("%s:%s:%s", input.ModelUUID, input.SpaceName, input.Zone))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *subnetsDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(d.subCtx, LogDataSourceSubnets, msg, additionalFields...)
}
