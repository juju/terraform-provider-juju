// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/charmhub"
)

// Ensure the implementation satisfies the expected interfaces.
var _ datasource.DataSource = &charmDataSource{}

// NewCharmDataSource returns a new data source for a CharmHub charm.
func NewCharmDataSource() datasource.DataSource {
	return &charmDataSource{}
}

type charmDataSource struct {
}

type charmDataSourceModel struct {
	Name         types.String `tfsdk:"charm"`
	Base         types.String `tfsdk:"base"`
	Channel      types.String `tfsdk:"channel"`
	Architecture types.String `tfsdk:"architecture"`
	Revision     types.Int64  `tfsdk:"revision"`
	StoreURL     types.String `tfsdk:"store_url"`
	// Resources is a map that keys the resource name to the revision.
	Resources map[string]types.String `tfsdk:"resources"`
	// Provides and Requires are maps that key the endpoint name to the interface name.
	Provides map[string]types.String `tfsdk:"provides"`
	// Requires and Provides are maps that key the endpoint name to the interface name.
	Requires map[string]types.String `tfsdk:"requires"`
}

// Metadata returns the full data source name as used in Terraform plans.
func (d *charmDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_charm"
}

// Schema returns the schema for the juju_charm data source.
func (d *charmDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A data source that fetches charm metadata from CharmHub, " +
			"including the resolved revision, the names and interfaces of " +
			"all integrations the charm provides or requires.",
		Attributes: map[string]schema.Attribute{
			"charm": schema.StringAttribute{
				Description: "The name of the charm to look up.",
				Required:    true,
			},
			"store_url": schema.StringAttribute{
				Description: "Base URL of the charm store. Defaults to https://charmhub.io/.",
				Optional:    true,
			},
			"base": schema.StringAttribute{
				Description: "The OS base for the charm in the form os@channel, e.g. \"ubuntu@22.04\".",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z]+@[0-9]+(?:\.[0-9]+)*$`),
						"must be in the form os@channel, e.g. \"ubuntu@22.04\"",
					),
					stringvalidator.AlsoRequires(path.MatchRoot("channel")),
				},
			},
			"channel": schema.StringAttribute{
				Description: "The channel to resolve, e.g. \"3/stable\". Required when revision is set.",
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("base")),
				},
			},
			"architecture": schema.StringAttribute{
				Description: "The architecture of the charm, e.g. \"amd64\". Defaults to \"amd64\" when not set.",
				Optional:    true,
			},
			"revision": schema.Int64Attribute{
				Description: "The revision of the charm to fetch.",
				Optional:    true,
				Computed:    true,
			},
			"resources": schema.MapAttribute{
				Description: "OCI/file resources for the charm. Key is the resource name, value is the revision number.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"provides": schema.MapAttribute{
				Description: "Integrations provided by the charm. Key is the endpoint name, value is the interface name.",
				Computed:    true,
				ElementType: types.StringType,
			},
			"requires": schema.MapAttribute{
				Description: "Integrations required by the charm. Key is the endpoint name, value is the interface name.",
				Computed:    true,
				ElementType: types.StringType,
			},
		},
	}
}

// Read fetches charm info from CharmHub and populates the Terraform state.
func (d *charmDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data charmDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	storeURL := charmhub.ProductionURL
	if !data.StoreURL.IsNull() && data.StoreURL.ValueString() != "" {
		storeURL = data.StoreURL.ValueString()
	}

	client := charmhub.New(storeURL, nil)

	input := charmhub.CharmRefreshInput{
		Name:         data.Name.ValueString(),
		Base:         data.Base.ValueString(),
		Channel:      data.Channel.ValueString(),
		Architecture: data.Architecture.ValueString(),
	}
	if !data.Revision.IsNull() && !data.Revision.IsUnknown() {
		r := int(data.Revision.ValueInt64())
		input.Revision = &r
	}

	tflog.Debug(ctx, "Refreshing charm from CharmHub", map[string]interface{}{
		"charm":        input.Name,
		"channel":      input.Channel,
		"base":         input.Base,
		"architecture": input.Architecture,
		"revision":     input.Revision,
		"storeURL":     storeURL,
	})

	result, err := client.Refresh(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error fetching charm info from CharmHub",
			err.Error(),
		)
		return
	}

	data.Revision = types.Int64Value(int64(result.Revision))
	if result.Channel != "" {
		data.Channel = types.StringValue(result.Channel)
	}
	if data.Base.IsNull() || data.Base.IsUnknown() {
		data.Base = types.StringValue(result.Base)
	}

	resources := make(map[string]types.String, len(result.Resources))
	for _, res := range result.Resources {
		resources[res.Name] = types.StringValue(fmt.Sprintf("%d", res.Revision))
	}
	data.Resources = resources

	provides := make(map[string]types.String, len(result.Provides))
	for name, rel := range result.Provides {
		provides[name] = types.StringValue(rel.Interface)
	}
	data.Provides = provides

	requires := make(map[string]types.String, len(result.Requires))
	for name, rel := range result.Requires {
		requires[name] = types.StringValue(rel.Interface)
	}
	data.Requires = requires

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
