// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

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
	Name  types.String `tfsdk:"name"`
	Owner types.String `tfsdk:"owner"`
	UUID  types.String `tfsdk:"uuid"`
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
		MarkdownDescription: "Use the model data source to retrieve information about an existing Juju model. " +
			"This is useful when you need to reference model attributes such as the model UUID in other resources.\n\n" +
			"Models can be looked up either by their UUID or a combination of name and owner e.g. admin/myModel. " +
			"The owner is the user that created the model and can be found with the 'juju show-model' command.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the model.",
				Optional:    true,
			},
			"owner": schema.StringAttribute{
				Description: "The owner of the model.",
				Optional:    true,
			},
			"uuid": schema.StringAttribute{
				Description: "The UUID of the model.",
				Optional:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

// ConfigValidators returns the validators used to ensure the configuration is
// valid prior to running the data source.
// For the model data source either name + owner must be specified together
// or uuid specified alone.
func (r *modelDataSource) ConfigValidators(ctx context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.RequiredTogether(
			path.MatchRoot("name"),
			path.MatchRoot("owner"),
		),
		datasourcevalidator.Conflicting(
			path.MatchRoot("name"),
			path.MatchRoot("uuid"),
		),
		datasourcevalidator.Conflicting(
			path.MatchRoot("owner"),
			path.MatchRoot("uuid"),
		),
		datasourcevalidator.AtLeastOneOf(
			path.MatchRoot("name"),
			path.MatchRoot("uuid"),
		),
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

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	resp.Diagnostics = checkControllerMode(resp.Diagnostics, provider.Config, false)
	if resp.Diagnostics.HasError() {
		return
	}

	d.client = provider.Client
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

	var modelUUID string
	if data.UUID.ValueString() != "" {
		modelUUID = data.UUID.ValueString()
	} else {
		if data.Name.ValueString() == "" || data.Owner.ValueString() == "" {
			resp.Diagnostics.AddError("Invalid Attribute Combination", "When looking up a model by name, both the name and owner attributes must be set.")
			return
		}
		uuid, err := d.client.Models.ModelUUID(data.Name.ValueString(), data.Owner.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read model by name and owner, got error: %s", err))
			return
		}
		modelUUID = uuid
	}

	// Get current juju model data source values.
	model, err := d.client.Models.GetModel(modelUUID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read model by UUID, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read juju model %q data source", data.UUID))

	owner, err := names.ParseUserTag(model.OwnerTag)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse model owner tag %q, got error: %s", model.OwnerTag, err))
		return
	}

	// Save data into Terraform state
	data.UUID = types.StringValue(model.UUID)
	data.ID = types.StringValue(model.UUID)
	data.Name = types.StringValue(model.Name)
	data.Owner = types.StringValue(owner.Id())
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
