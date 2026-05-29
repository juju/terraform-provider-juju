// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

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

var _ datasource.DataSourceWithConfigure = &spaceDataSource{}

// NewSpaceDataSource returns a space data source.
func NewSpaceDataSource() datasource.DataSource {
	return &spaceDataSource{}
}

type spaceDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type spaceDataSourceModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	Name      types.String `tfsdk:"name"`

	// ID required by the testing framework.
	ID types.String `tfsdk:"id"`
}

func (d *spaceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (d *spaceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju space.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the space exists.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the space.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidSpace, "must be a valid space name"),
				},
			},
			"id": schema.StringAttribute{
				Description: "The identifier of the space data source. Format: <model_uuid>:<name>",
				Computed:    true,
			},
		},
	}
}

func (d *spaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderDataForDataSource(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceSpace)
}

func (d *spaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "space")
		return
	}

	var data spaceDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := juju.ReadSpaceInput{
		ModelUUID: data.ModelUUID.ValueString(),
		Name:      data.Name.ValueString(),
	}
	space, err := d.client.Spaces.ReadSpace(ctx, &input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read space data source, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read space data source %q", input.Name), map[string]any{
		"model_uuid": input.ModelUUID,
		"name":       input.Name,
	})

	data.ModelUUID = types.StringValue(input.ModelUUID)
	data.Name = types.StringValue(space.Name)
	data.ID = types.StringValue(newSpaceResourceID(input.ModelUUID, space.Name))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *spaceDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(d.subCtx, LogDataSourceSpace, msg, additionalFields...)
}
