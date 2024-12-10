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

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSourceWithConfigure = &applicationDataSource{}

// NewApplicationDataSource returns a new data source for a Juju application.
func NewApplicationDataSource() datasource.DataSourceWithConfigure {
	return &applicationDataSource{}
}

type applicationDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type applicationDataSourceModel struct {
	ApplicationName types.String `tfsdk:"name"`
	ModelName       types.String `tfsdk:"model"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *applicationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// Schema returns the schema for the application data source.
func (d *applicationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source that represents a single Juju application deployment from a charm.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "Name of the application deployment.",
				Required:    true,
			},
			"model": schema.StringAttribute{
				Description: "The name of the model where the application is deployed.",
				Required:    true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (d *applicationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceApplication)
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
// Take the juju api input from the ID, it may not exist in the plan.
// Only set optional values if they exist.
func (d *applicationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "application")
		return
	}
	var data applicationDataSourceModel

	// Read Terraform prior data into the application
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	appName := data.ApplicationName.ValueString()
	modelName := data.ModelName.ValueString()
	d.trace("Read", map[string]interface{}{
		"Model": modelName,
		"Name":  appName,
	})

	response, err := d.client.Applications.ReadApplication(&juju.ReadApplicationInput{
		ModelName: modelName,
		AppName:   appName,
	})
	if err != nil {
		resp.Diagnostics.Append(handleApplicationNotFoundError(ctx, err, &resp.State)...)
		return
	}
	if response == nil {
		return
	}
	d.trace("read application", map[string]interface{}{"resource": appName, "response": response})

	data.ApplicationName = types.StringValue(appName)
	data.ModelName = types.StringValue(modelName)

	d.trace("Found", applicationDataSourceModelForLogging(ctx, &data))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *applicationDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-model", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-model","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceApplication, msg, additionalFields...)
}

func applicationDataSourceModelForLogging(_ context.Context, app *applicationDataSourceModel) map[string]interface{} {
	value := map[string]interface{}{
		"application-name": app.ApplicationName.ValueString(),
		"model":            app.ModelName.ValueString(),
	}
	return value
}
