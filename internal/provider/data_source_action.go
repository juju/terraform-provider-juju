// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &actionDataSource{}
var _ datasource.DataSourceWithConfigure = &actionDataSource{}

// NewActionDataSource returns a new action data source.
func NewActionDataSource() datasource.DataSource {
	return &actionDataSource{}
}

type actionDataSource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for the
	// action data source.
	subCtx context.Context
}

// actionDataSourceModel is the model for the juju_action data source.
type actionDataSourceModel struct {
	// ModelUUID is the UUID of the model where the action was run.
	ModelUUID types.String `tfsdk:"model_uuid"`
	// ActionID is the ID of the action whose result is fetched.
	ActionID types.String `tfsdk:"action_id"`
	// Output is the output of the action as a JSON string. The consumer
	// can use jsondecode() to extract values from it.
	Output types.String `tfsdk:"output"`
	// ID required by the testing framework.
	ID types.String `tfsdk:"id"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *actionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

// Schema defines the schema for the action data source.
func (d *actionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju action. The action's result is fetched via its ID and awaited during the data " +
			"source read. This allows actions to be run outside of Terraform and their results consumed by other resources.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the action was run.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"action_id": schema.StringAttribute{
				Description: "The ID of the action whose result is fetched.",
				Required:    true,
			},
			"output": schema.StringAttribute{
				Description: "The output of the action as a JSON string. Use jsondecode() to extract values from it.",
				Computed:    true,
			},
			// ID required by the testing framework.
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (d *actionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceAction)
}

// Read fetches the action result by its ID and awaits its completion.
func (d *actionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "action")
		return
	}

	var data actionDataSourceModel

	// Read Terraform configuration data into the model.
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID := data.ModelUUID.ValueString()
	actionID := data.ActionID.ValueString()
	d.trace(fmt.Sprintf("reading juju action %q data source", actionID))

	// Wait for the action to complete and populate the output.
	actionResult, err := waitActionResult(ctx, d.client, func(msg string, additionalFields ...map[string]interface{}) {
		tflog.SubsystemDebug(d.subCtx, LogDataSourceAction, msg, additionalFields...)
	}, modelUUID, actionID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for action %q to complete: %s", actionID, err))
		return
	}

	data.Output, err = actionResultToOutput(actionResult)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert action output: %s", err))
		return
	}

	// "id" is required for tests. Data sources cannot be imported by
	// terraform, so it need not be unique.
	data.ID = types.StringValue(fmt.Sprintf("%s:%s", modelUUID, actionID))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *actionDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceAction, msg, additionalFields...)
}
