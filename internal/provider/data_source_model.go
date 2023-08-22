// Copyright 2023 Canonical Ltd.
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

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSourceWithConfigure = &modelDataSource{}

func NewModelDataSource() datasource.DataSourceWithConfigure {
	return &modelDataSource{}
}

type modelDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type modelDataSourceModel struct {
	Name types.String `tfsdk:"name"`
	UUID types.String `tfsdk:"uuid"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *modelDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

// Schema returns the schema for the model data source.
func (d *modelDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Model.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the model.",
				Required:    true,
			},
			"uuid": schema.StringAttribute{
				Description: "The UUID of the model.",
				Computed:    true,
			},
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (d *modelDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*juju.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *juju.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceModel)
}

// Read is called when the provider must read data source values in
// order to update state. Config values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (d *modelDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "model")
		return
	}

	var data modelDataSourceModel

	// Read Terraform configuration data into the model.
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current juju model data source values.
	model, err := d.client.Models.GetModelByName(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read model, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read juju model %q data source", data.Name))

	// Save data into Terraform state
	data.Name = types.StringValue(model.Name)
	data.UUID = types.StringValue(model.UUID)
	data.ID = types.StringValue(model.UUID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *modelDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-model", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-model","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceModel, msg, additionalFields...)
}
