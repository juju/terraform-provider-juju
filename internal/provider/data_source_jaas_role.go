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

type jaasRoleDataSource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// NewJAASRoleDataSource returns a new JAAS role data source instance.
func NewJAASRoleDataSource() datasource.DataSource {
	return &jaasRoleDataSource{}
}

type jaasRoleDataSourceModel struct {
	Name types.String `tfsdk:"name"`
	UUID types.String `tfsdk:"uuid"`
}

// Metadata returns the metadata for the JAAS role data source.
func (d *jaasRoleDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_role"
}

// Schema defines the schema for JAAS roles.
func (d *jaasRoleDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju JAAS Role.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the role.",
				Required:    true,
			},
			"uuid": schema.StringAttribute{
				Description: "The UUID of the role. The UUID is used to reference roles in other resources.",
				Computed:    true,
			},
		},
	}
}

// Configure sets up the JAAS role data source with the provider data.
func (d *jaasRoleDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*juju.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceJAASRole)
}

// Read updates the role data source with the latest data from JAAS.
func (d *jaasRoleDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "jaas-role")
		return
	}

	var data jaasRoleDataSourceModel

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the role with the latest data from JAAS
	role, err := d.client.Jaas.ReadRoleByName(data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read role, got error: %v", err))
		return
	}
	data.UUID = types.StringValue(role.UUID)
	d.trace(fmt.Sprintf("read role %q data source", data.Name))

	// Save the updated role back to the state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *jaasRoleDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-jaas-role", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-jaas-role","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceJAASRole, msg, additionalFields...)
}
