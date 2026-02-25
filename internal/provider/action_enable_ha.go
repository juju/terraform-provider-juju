// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/action"
	actionschema "github.com/hashicorp/terraform-plugin-framework/action/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ action.Action = (*enableHAAction)(nil)
var _ action.ActionWithConfigure = (*enableHAAction)(nil)

// enableHAActionModel is the configuration model for the enable_ha action.
type enableHAActionModel struct {
	// APIAddresses are the API endpoints of the target controller
	// (juju_controller.<name>.api_addresses).
	APIAddresses types.List `tfsdk:"api_addresses"`
	// CACert is the CA certificate for the target controller
	// (juju_controller.<name>.ca_cert).
	CACert types.String `tfsdk:"ca_cert"`
	// Username is the admin username for the target controller
	// (juju_controller.<name>.username).
	Username types.String `tfsdk:"username"`
	// Password is the admin password for the target controller
	// (juju_controller.<name>.password).
	Password types.String `tfsdk:"password"`
	// Units is the desired number of controller units (must be odd and >= 3).
	Units types.Int64 `tfsdk:"units"`
	// Constraints is an optional placement constraint for new controller units.
	Constraints types.String `tfsdk:"constraints"`
	// To is an optional list of placement directives for new controller units
	// (e.g. ["lxd:0", "lxd:1"]). When omitted, Juju selects placement
	// automatically.
	To types.List `tfsdk:"to"`
}

// enableHAAction implements the juju_enable_ha Terraform action.
type enableHAAction struct {
	haClient *juju.EnableHAClient
}

// NewEnableHAAction returns a factory function suitable for use in
// provider.ProviderWithActions.Actions().
func NewEnableHAAction() action.Action {
	return &enableHAAction{}
}

// Metadata returns the full type name of the action.
func (a *enableHAAction) Metadata(_ context.Context, req action.MetadataRequest, resp *action.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_enable_ha"
}

// Schema defines the configuration attributes the action accepts.
func (a *enableHAAction) Schema(_ context.Context, _ action.SchemaRequest, resp *action.SchemaResponse) {
	resp.Schema = actionschema.Schema{
		Description: "Enables high availability (HA) on a Juju controller by " +
			"ensuring the desired number of controller units are running.",
		Attributes: map[string]actionschema.Attribute{
			"api_addresses": actionschema.ListAttribute{
				Description: "API addresses of the target controller. Reference from " +
					"the controller resource: juju_controller.<name>.api_addresses.",
				ElementType: types.StringType,
				Required:    true,
			},
			"ca_cert": actionschema.StringAttribute{
				Description: "CA certificate of the target controller. Reference from " +
					"the controller resource: juju_controller.<name>.ca_cert.",
				Required: true,
			},
			"username": actionschema.StringAttribute{
				Description: "Admin username for the target controller. Reference from " +
					"the controller resource: juju_controller.<name>.username.",
				Required: true,
			},
			"password": actionschema.StringAttribute{
				Description: "Admin password for the target controller. Reference from " +
					"the controller resource: juju_controller.<name>.password.",
				Required: true,
			},
			"units": actionschema.Int64Attribute{
				Description: "Desired number of controller units. Must be an odd " +
					"number and at least 3. Increasing the number of units is " +
					"supported; decreasing is not possible via this action and " +
					"must be done manually via the Juju CLI.",
				Required: true,
			},
			"constraints": actionschema.StringAttribute{
				Description: "Optional placement constraints for newly provisioned " +
					"controller units (e.g. \"mem=8G cores=4\").",
				Optional: true,
			},
			"to": actionschema.ListAttribute{
				Description: "Optional list of placement directives for new controller " +
					"units (e.g. [\"lxd:0\", \"lxd:1\"]). When omitted, Juju selects " +
					"placement automatically.",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

// Configure receives the provider-level data and initialises the EnableHAClient.
// It uses getProviderDataForAction to validate that the provider is running in
// controller mode, which is required for HA operations.
func (a *enableHAAction) Configure(_ context.Context, req action.ConfigureRequest, resp *action.ConfigureResponse) {
	// req.ProviderData is nil during offline operations (e.g. terraform validate).
	if req.ProviderData == nil {
		return
	}

	_, diags := getProviderDataForAction(req, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	a.haClient = juju.NewEnableHAClient()
}

// Invoke builds the connection details from the action config and runs the
// enable-HA operation.
func (a *enableHAAction) Invoke(ctx context.Context, req action.InvokeRequest, resp *action.InvokeResponse) {
	var data enableHAActionModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var apiAddresses []string
	resp.Diagnostics.Append(data.APIAddresses.ElementsAs(ctx, &apiAddresses, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	haClient := a.haClient
	if haClient == nil {
		haClient = juju.NewEnableHAClient()
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf(
			"Enabling HA with %d units on controller %v ...",
			data.Units.ValueInt64(),
			apiAddresses,
		),
	})

	var placement []string
	if !data.To.IsNull() && !data.To.IsUnknown() {
		resp.Diagnostics.Append(data.To.ElementsAs(ctx, &placement, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	input := juju.EnableHAInput{
		ConnInfo: juju.ControllerConnectionInformation{
			Addresses: apiAddresses,
			CACert:    data.CACert.ValueString(),
			Username:  data.Username.ValueString(),
			Password:  data.Password.ValueString(),
		},
		Constraints: data.Constraints.ValueString(),
		Units:       int(data.Units.ValueInt64()),
		To:          placement,
	}

	if err := haClient.EnableHA(ctx, input); err != nil {
		resp.Diagnostics.AddError(
			"Enable HA Error",
			fmt.Sprintf("Failed to enable HA on controller %v: %s", apiAddresses, err),
		)
		return
	}

	resp.SendProgress(action.InvokeProgressEvent{
		Message: fmt.Sprintf(
			"High availability enabled with %d units on controller %v.",
			data.Units.ValueInt64(),
			apiAddresses,
		),
	})
}
