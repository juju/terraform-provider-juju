// Copyright 2023 Canonical Ltd.
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
var _ datasource.DataSourceWithConfigure = &offerDataSource{}

func NewOfferDataSource() datasource.DataSource {
	return &offerDataSource{}
}

type offerDataSource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

// offerDataSourceModel is the juju data stored by terraform.
// tfsdk must match offer data source schema attribute names.
type offerDataSourceModel struct {
	ApplicationName types.String `tfsdk:"application_name"`
	Endpoint        types.String `tfsdk:"endpoint"`
	ModelName       types.String `tfsdk:"model"`
	OfferName       types.String `tfsdk:"name"`
	OfferURL        types.String `tfsdk:"url"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (d *offerDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offer"
}

func (d *offerDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source representing a Juju Offer.",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "The offer URL.",
				Required:    true,
			},
			"model": schema.StringAttribute{
				Description: "The name of the model to operate in.",
				Computed:    true,
				// TODO hml 21-aug-2023
				// Is model necessary at all?
			},
			"name": schema.StringAttribute{
				Description: "The name of the offer.",
				Computed:    true,
			},
			"application_name": schema.StringAttribute{
				Description: "The name of the application.",
				Computed:    true,
			},
			"endpoint": schema.StringAttribute{
				Description: "The endpoint name.",
				Computed:    true,
			},
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *offerDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
	d.subCtx = tflog.NewSubsystem(ctx, LogDataSourceOffer)
}

func (d *offerDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if d.client == nil {
		addDSClientNotConfiguredError(&resp.Diagnostics, "offer")
		return
	}

	var data offerDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get current juju machine data source values .
	offer, err := d.client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: data.OfferURL.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read offer, got error: %s", err))
		return
	}
	d.trace(fmt.Sprintf("read juju offer %q data source", data.OfferName))

	// Save data into Terraform state
	data.ApplicationName = types.StringValue(offer.ApplicationName)
	data.Endpoint = types.StringValue(offer.Endpoint)
	data.ModelName = types.StringValue(offer.ModelName)
	data.OfferName = types.StringValue(offer.Name)
	data.OfferURL = types.StringValue(offer.OfferURL)
	data.ID = types.StringValue(offer.OfferURL)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *offerDataSource) trace(msg string, additionalFields ...map[string]interface{}) {
	if d.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "datasource-offer", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"juju.datasource-offer","foo":123}
	tflog.SubsystemTrace(d.subCtx, LogDataSourceOffer, msg, additionalFields...)
}
