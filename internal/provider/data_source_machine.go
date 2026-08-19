// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/clock"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &machineDataSource{}

const (
	waitForIPAddressesKey = "wait_for_ip_addresses"
	ipAddressesKey        = "ip_addresses"
)

// NewMachineDataSource returns a new machine data source.
func NewMachineDataSource() datasource.DataSourceWithConfigure {
	return &machineDataSource{}
}

type machineDataSource struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

type machineDataSourceModel struct {
	ModelUUID          types.String `tfsdk:"model_uuid"`
	MachineID          types.String `tfsdk:"machine_id"`
	WaitForIPAddresses types.List   `tfsdk:"wait_for_ip_addresses"`
	IPAddresses        types.List   `tfsdk:"ip_addresses"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *machineDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

// Schema defines the schema for the machine data source.
func (d *machineDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Machine.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"machine_id": schema.StringAttribute{
				Description: "The Juju id of the machine.",
				Required:    true,
			},
			waitForIPAddressesKey: schema.ListAttribute{
				Description: "A list of IP address conditions the provider waits for before completing " +
					"the read. Each element must be one of: a CIDR (e.g. \"10.0.10.0/24\"), " +
					"or the aliases \"public\", \"private\", \"any\". The matching IP addresses are populated " +
					"in the 'ip_addresses' computed field, in the same order as this list. Conditions are evaluated " +
					"from left to right, so put narrow CIDRs first, followed by broader CIDRs or \"public\"/\"private\", " +
					"and \"any\" last.",
				Optional:    true,
				ElementType: types.StringType,
				Validators: []validator.List{
					listvalidator.ValueStringsAre(ipAddressConditionValidator{}),
				},
			},
			ipAddressesKey: schema.ListAttribute{
				Description: "If `wait_for_ip_addresses` is set it will contain the IP addresses of the machine matching the 'wait_for_ip_addresses' " +
					"conditions, in the same order. If not set this field will contain the IPs fetched when the machine is read.",
				Computed:    true,
				ElementType: types.StringType,
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

	provider, diags := getProviderDataForDataSource(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
	d.config = provider.Config
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
	machineID := data.MachineID.ValueString()
	d.trace(fmt.Sprintf("reading juju machine %q data source", machineID))
	var conditions []string
	resp.Diagnostics.Append(data.WaitForIPAddresses.ElementsAs(ctx, &conditions, true)...)
	if resp.Diagnostics.HasError() {
		return
	}
	modelUUID := data.ModelUUID.ValueString()
	asserts := []wait.Assert[*juju.ReadMachineResponse]{}
	if len(conditions) > 0 {
		asserts = append(asserts, assertIPAddressesFor(conditions))
	}
	readResponse, err := wait.WaitFor(wait.WaitForCfg[*juju.ReadMachineInput, *juju.ReadMachineResponse]{
		Context: ctx,
		GetData: d.client.Machines.ReadMachine,
		Input: &juju.ReadMachineInput{
			ModelUUID: modelUUID,
			ID:        machineID,
		},
		DataAssertions: asserts,
		NonFatalErrors: []error{juju.RetryReadError, juju.ConnectionRefusedError, juju.ErrNoMatchingIPAddress},
		RetryConf: &wait.RetryConf{
			MaxDuration: d.config.DefaultCreateTimeout,
			Delay:       juju.ReadModelDefaultInterval,
			Clock:       clock.WallClock,
		},
		Logf: d.trace,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for machine %q readiness, got error: %s", machineID, err))
		return
	}

	ipAddresses := readResponse.IPAddresses
	if len(conditions) > 0 {
		ipAddresses, err = juju.MatchIPAddresses(ipAddresses, conditions)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to match machine IP addresses for machine %q, got error: %s", machineID, err))
			return
		}
	}
	ipAddressesValue, diag := types.ListValueFrom(ctx, types.StringType, ipAddresses)
	if diag.HasError() {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert machine IP addresses for machine %q, got error: %v", machineID, diag.Errors()))
		return
	}
	data.IPAddresses = ipAddressesValue
	// machine_id is not unique, however it matches the
	// SDK value used. "id" is required for tests.
	data.ID = types.StringValue(machineID)

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

// assertIPAddressesFor returns a wait assertion ensuring that the machine's
// reported IP addresses can satisfy all the given wait-for-ip-addresses
// conditions. ErrNoMatchingIPAddress is treated as retriable by WaitFor (via
// NonFatalErrors); any other error is fatal.
func assertIPAddressesFor(conditions []string) wait.Assert[*juju.ReadMachineResponse] {
	return func(respFromAPI *juju.ReadMachineResponse) error {
		_, err := juju.MatchIPAddresses(respFromAPI.IPAddresses, conditions)
		return err
	}
}
