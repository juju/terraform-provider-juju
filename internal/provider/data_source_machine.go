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
var _ datasource.DataSource = &machineDataSource{}

func NewMachineDataSource() datasource.DataSourceWithConfigure {
	return &machineDataSource{}
}

type machineDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type machineDataSourceModel struct {
	Model     types.String `tfsdk:"model"`
	MachineID types.String `tfsdk:"machine_id"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *machineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

func (d *machineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Machine.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model.",
				Required:    true,
			},
			"machine_id": schema.StringAttribute{
				Description: "The Juju id of the machine.",
				Required:    true,
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
func (d *machineDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceMachine)
}

// Read is called when the provider must read data source values in
// order to update state. Config values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (d *machineDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "machine")
		return
	}

	var data machineDataSourceModel

	// Read Terraform configuration data into the model.
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current juju machine data source values.
	// "id" matches previous provider values however is not
	// unique and is only used for tests. Data sources cannot
	// be imported by terraform.
	machine_id := data.MachineID.ValueString()
	d.trace(fmt.Sprintf("reading juju machine %q data source", machine_id))

	// Verify the machine exists in the model provided
	if _, err := d.client.Machines.ReadMachine(
		juju.ReadMachineInput{
			ModelName: data.Model.ValueString(),
			ID:        machine_id,
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read machine %q, got error: %s", machine_id, err))
		return
	}

	// machine_id is not unique, however it matches the
	// SDK value used. "id" is required for tests.
	data.ID = types.StringValue(machine_id)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *machineDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-machine", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-machine","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceMachine, msg, additionalFields...)
}
