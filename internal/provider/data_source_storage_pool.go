// Copyright 2025 Canonical Ltd.
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

var _ datasource.DataSourceWithConfigure = &storagePoolDataSource{}

// NewStoragePoolDataSource returns a storage pool data source.
func NewStoragePoolDataSource() datasource.DataSource {
	return &storagePoolDataSource{}
}

type storagePoolDataSource struct {
	client *juju.Client

	subCtx context.Context
}

type storagePoolDataSourceModel struct {
	Name      types.String `tfsdk:"name"`
	ModelUUID types.String `tfsdk:"model_uuid"`
}

// Metadata implements datasource.DataSourceWithConfigure.Metadata.
func (d *storagePoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_pool"
}

// Schema implements datasource.DataSourceWithConfigure.Schema.
func (d *storagePoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Storage Pool.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The uuid of the model containing the storage pool.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the storage pool.",
				Required:    true,
			},
		},
	}
}

// Configure implements datasource.DataSourceWithConfigure.Configure.
func (d *storagePoolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderDataForDataSource(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceStoragePool)
}

// Read implements datasource.DataSourceWithConfigure.Read.
func (d *storagePoolDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "storage pool")
		return
	}

	var data storagePoolDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := juju.GetStoragePoolInput{
		ModelUUID: data.ModelUUID.ValueString(),
		PoolName:  data.Name.ValueString(),
	}
	output, err := d.client.Storage.GetPool(input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read storage pool, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read storage pool data source %q", data.Name.ValueString()), map[string]interface{}{
		"model-uuid": data.ModelUUID.ValueString(),
		"name":       data.Name.ValueString(),
	})

	data.Name = types.StringValue(output.Pool.Name)
	data.ModelUUID = types.StringValue(input.ModelUUID)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *storagePoolDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	tflog.SubsystemTrace(d.subCtx, LogDataSourceStoragePool, msg, additionalFields...)
}
