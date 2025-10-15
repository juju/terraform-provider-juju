// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ datasource.DataSourceWithConfigure = &storagePoolDataSource{}

func NewStoragePoolDataSource() datasource.DataSource {
	return &storagePoolDataSource{}
}

type storagePoolDataSource struct {
	client *juju.Client

	subCtx context.Context
}

type storagePoolDataSourceModel struct {
	Name       types.String `tfsdk:"name"`
	ModelName  types.String `tfsdk:"model_name"`
	ModelOwner types.String `tfsdk:"model_owner"`
	ModelUUID  types.String `tfsdk:"model_uuid"`
}

// Metadata implements datasource.DataSourceWithConfigure.Metadata.
func (d *storagePoolDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_pool"
}

// Schema implements datasource.DataSourceWithConfigure.Schema.
func (d *storagePoolDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Storage Pool.",
		MarkdownDescription: "Use the storage pool data source to retrieve information about an existing Juju storage pool. " +
			"This is useful when you need to reference storage pool attributes such as the pool attributes in other resources. " +
			"Storage pools can be looked up by their name with a model UUID or a model owner and model name e.g. admin/myModel. " +
			"The owner is the user that created the model and can be found with the 'juju show-model' command.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the storage pool.",
				Optional:    true,
			},
			"model_uuid": schema.StringAttribute{
				Description: "The uuid of the model containing the storage pool.",
				Optional:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"model_name": schema.StringAttribute{
				Description: "The name of the model.",
				Optional:    true,
			},
			"model_owner": schema.StringAttribute{
				Description: "The owner of the model.",
				Optional:    true,
			},
		},
	}
}

// ConfigValidators returns the validators used to ensure the configuration is
// valid prior to running the data source.
// For the storage pool data source either model_name + model_owner must be specified together
// or model_uuid specified alone.
func (r *storagePoolDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.RequiredTogether(
			path.MatchRoot("model_name"),
			path.MatchRoot("model_owner"),
		),
		datasourcevalidator.Conflicting(
			path.MatchRoot("model_name"),
			path.MatchRoot("model_uuid"),
		),
		datasourcevalidator.Conflicting(
			path.MatchRoot("model_owner"),
			path.MatchRoot("model_uuid"),
		),
		datasourcevalidator.AtLeastOneOf(
			path.MatchRoot("model_name"),
			path.MatchRoot("model_uuid"),
		),
	}
}

// Configure implements datasource.DataSourceWithConfigure.Configure.
func (d *storagePoolDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
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

	var modelUUID string
	if data.ModelUUID.ValueString() != "" {
		modelUUID = data.ModelUUID.ValueString()
	} else {
		if data.ModelName.ValueString() == "" || data.ModelOwner.ValueString() == "" {
			resp.Diagnostics.AddError("Invalid Attribute Combination", "When looking up a model by name, both the name and owner attributes must be set.")
			return
		}
		uuid, err := d.client.Models.ModelUUID(data.ModelName.ValueString(), data.ModelOwner.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read model by name and owner, got error: %s", err))
			return
		}
		modelUUID = uuid
	}

	input := juju.GetStoragePoolInput{
		ModelUUID: modelUUID,
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
