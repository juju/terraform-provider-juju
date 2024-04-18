// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

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
var _ datasource.DataSourceWithConfigure = &secretDataSource{}

func NewSecretDataSource() datasource.DataSource {
	return &secretDataSource{}
}

type secretDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

// secretDataSourceModel is the juju data stored by terraform.
// tfsdk must match secret data source schema attribute names.
type secretDataSourceModel struct {
	// Model to which the secret belongs.
	Model types.String `tfsdk:"model"`
	// Name of the secret to be updated or removed.
	Name types.String `tfsdk:"name"`
	// SecretId is the ID of the secret.
	SecretId types.String `tfsdk:"secret_id"`
}

// Metadata returns the full data source name as used in terraform plans.
func (d *secretDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

// Schema returns the schema for the model data source.
func (d *secretDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Secret.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model containing the secret.",
				Required:    true,
			},
			"name": schema.StringAttribute{
				Description: "The name of the secret.",
				Required:    true,
			},
			"secret_id": schema.StringAttribute{
				Description: "The ID of the secret.",
				Computed:    true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (d *secretDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceSecret)
}

// Read is called when the provider must read data source values in
// order to update state. Config values should be read from the
// ReadRequest and new state values set on the ReadResponse.
func (d *secretDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "secret")
		return
	}

	var data secretDataSourceModel

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readSecretInput := juju.ReadSecretInput{
		ModelName: data.Model.ValueString(),
	}
	if data.SecretId.ValueString() == "" {
		readSecretInput.Name = data.Name.ValueStringPointer()
	} else {
		readSecretInput.SecretId = data.SecretId.ValueString()
	}

	readSecretOutput, err := d.client.Secrets.ReadSecret(&readSecretInput)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read secret data source %q", data.SecretId))

	data.SecretId = types.StringValue(readSecretOutput.SecretId)

	// Save state into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *secretDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-secret", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-secret","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceSecret, msg, additionalFields...)
}
