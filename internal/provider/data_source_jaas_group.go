// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type jaasGroupDataSource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// NewJAASGroupDataSource returns a new JAAS group data source instance.
func NewJAASGroupDataSource() datasource.DataSource {
	return &jaasGroupDataSource{}
}

type jaasGroupDataSourceModel struct {
	Name types.String `tfsdk:"name"`
	UUID types.String `tfsdk:"uuid"`
}

// Metadata returns the metadata for the JAAS group data source.
func (d *jaasGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_group"
}

// Schema defines the schema for JAAS groups.
func (d *jaasGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju JAAS Group.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the group.",
				Required:    true,
			},
			"uuid": schema.StringAttribute{
				Description: "The UUID of the group.",
				Computed:    true,
			},
		},
	}
}

// Configure sets up the JAAS group data source with the provider data.
func (d *jaasGroupDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderDataForDataSource(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceJAASGroup)
}

// Read updates the group data source with the latest data from JAAS.
func (d *jaasGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "jaas-group")
		return
	}

	var data jaasGroupDataSourceModel

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the group with the latest data from JAAS
	group, err := d.client.Jaas.ReadGroupByName(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read group, got error: %v", err))
		return
	}
	data.UUID = types.StringValue(group.UUID)
	d.trace(fmt.Sprintf("read group %q data source", data.Name))

	// Save the updated group back to the state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *jaasGroupDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-jaas-group", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-jaas-group","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceJAASGroup, msg, additionalFields...)
}
