// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &cloudResource{}
var _ resource.ResourceWithConfigure = &cloudResource{}

func NewCloudResource() resource.Resource {
	return &cloudResource{}
}

type cloudResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for clouds.
	subCtx context.Context
}

type cloudRegionModel struct {
	Name             types.String `tfsdk:"name"`
	Endpoint         types.String `tfsdk:"endpoint"`
	IdentityEndpoint types.String `tfsdk:"identity_endpoint"`
	StorageEndpoint  types.String `tfsdk:"storage_endpoint"`
}

type cloudResourceModel struct {
	Name             types.String `tfsdk:"name"`
	Type             types.String `tfsdk:"type"`
	AuthTypes        types.List   `tfsdk:"auth_types"`
	Endpoint         types.String `tfsdk:"endpoint"`
	IdentityEndpoint types.String `tfsdk:"identity_endpoint"`
	StorageEndpoint  types.String `tfsdk:"storage_endpoint"`
	CACertificates   types.List   `tfsdk:"ca_certificates"`
	Regions          types.List   `tfsdk:"regions"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Configure is used to configure the cloud resource.
func (r *cloudResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceCloud)
}

// Metadata returns the metadata for the cloud resource.
func (r *cloudResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_cloud"
}

// Schema returns the schema for the cloud resource.
func (r *cloudResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju Cloud for an existing controller.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the cloud for Juju. Changing this value will cause the cloud to be destroyed and recreated by terraform.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Description: "The type of the cloud (e.g., 'openstack', 'aws', 'maas').",
				Required:    true,
			},
			"auth_types": schema.ListAttribute{
				Description: "List of supported authentication types by the cloud.",
				ElementType: types.StringType,
				Required:    true,
			},
			"endpoint": schema.StringAttribute{
				Description: "Optional global endpoint for the cloud.",
				Optional:    true,
			},
			"identity_endpoint": schema.StringAttribute{
				Description: "Optional global identity endpoint for the cloud.",
				Optional:    true,
			},
			"storage_endpoint": schema.StringAttribute{
				Description: "Optional global storage endpoint for the cloud.",
				Optional:    true,
			},
			"ca_certificates": schema.ListAttribute{
				Description: "List of PEM-encoded X509 certificates for the cloud.",
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
				// Juju doesn't validate the certificates on add/update, but we can at least
				// ensure they are valid PEM-encoded certs here.
				Validators: []validator.List{ValidateCACertificatesPEM()},
			},
			// All clouds must have at least one default region. We want to allow users to optionally use
			// the default region, as such, we're adhering to the Juju requirement here.
			"regions": schema.ListNestedAttribute{
				Description: "List of regions for the cloud. The first entry is the default region.",
				Computed:    true,
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":              schema.StringAttribute{Required: true, Description: "Name of the region."},
						"endpoint":          schema.StringAttribute{Optional: true, Description: "Region-specific endpoint."},
						"identity_endpoint": schema.StringAttribute{Optional: true, Description: "Region-specific identity endpoint."},
						"storage_endpoint":  schema.StringAttribute{Optional: true, Description: "Region-specific storage endpoint."},
					},
				},
				Default: defaultRegionForCloud{},
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Create adds a new cloud to the controller.
func (r *cloudResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "cloud", "create")
		return
	}

	var plan cloudResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regions, diags := expandRegions(ctx, plan.Regions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(regions) == 0 {
		resp.Diagnostics.AddError("Plan Error", "Field `regions` must contain at least one region (the first is the default).")
		return
	}
	authTypes, diags2 := expandStringList(ctx, plan.AuthTypes)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}
	cacerts, diags3 := expandStringList(ctx, plan.CACertificates)
	resp.Diagnostics.Append(diags3...)
	if resp.Diagnostics.HasError() {
		return
	}

	// convert []string auth types to jujucloud.AuthTypes
	at := make(jujucloud.AuthTypes, len(authTypes))
	for i, s := range authTypes {
		at[i] = jujucloud.AuthType(s)
	}

	input := juju.AddCloudInput{
		Name:             plan.Name.ValueString(),
		Type:             plan.Type.ValueString(),
		Description:      "",
		AuthTypes:        at,
		Endpoint:         plan.Endpoint.ValueString(),
		IdentityEndpoint: plan.IdentityEndpoint.ValueString(),
		StorageEndpoint:  plan.StorageEndpoint.ValueString(),
		Regions:          regions,
		CACertificates:   cacerts,
		Force:            false,
	}

	if err := r.client.Clouds.AddCloud(input); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create cloud, got error %s", err))
		return
	}

	r.trace(fmt.Sprintf("Created cloud %s", plan.Name.ValueString()))

	plan.ID = types.StringValue(plan.Name.ValueString())
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read reads the current state of the cloud.
func (r *cloudResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "cloud", "read")
		return
	}

	var state cloudResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	out, err := r.client.Clouds.ReadCloud(juju.ReadCloudInput{Name: state.Name.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read cloud, got error %s", err))
		return
	}

	state.Name = types.StringValue(out.Name)
	state.Type = types.StringValue(out.Type)

	var dErr diag.Diagnostics
	state.AuthTypes, dErr = types.ListValueFrom(ctx, types.StringType, out.AuthTypes)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	// Maintain nullability: if no CA certs are set server-side after create, and we planned with no ca certs,
	// keep this attribute null so Terraform does not plan a change from null -> [] or vice versa.
	if len(out.CACertificates) == 0 {
		state.CACertificates = types.ListNull(types.StringType)
	} else {
		state.CACertificates, dErr = types.ListValueFrom(ctx, types.StringType, out.CACertificates)
		if dErr.HasError() {
			resp.Diagnostics.Append(dErr...)
			return
		}
	}

	// Alex: Must be a better way than this?
	if lst, d := flattenRegions(ctx, out.Regions); !d.HasError() {
		state.Regions = lst
	} else {
		resp.Diagnostics.Append(d...)
		return
	}

	// Check for "" and maintain nullability.
	if out.Endpoint == "" {
		state.Endpoint = types.StringNull()
	} else {
		state.Endpoint = types.StringValue(out.Endpoint)
	}
	if out.IdentityEndpoint == "" {
		state.IdentityEndpoint = types.StringNull()
	} else {
		state.IdentityEndpoint = types.StringValue(out.IdentityEndpoint)
	}
	if out.StorageEndpoint == "" {
		state.StorageEndpoint = types.StringNull()
	} else {
		state.StorageEndpoint = types.StringValue(out.StorageEndpoint)
	}

	state.ID = types.StringValue(out.Name)

	r.trace(fmt.Sprintf("Read cloud %s", state.Name.ValueString()))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the cloud on the controller.
func (r *cloudResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "cloud", "update")
		return
	}
	var plan cloudResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	regions, diags := expandRegions(ctx, plan.Regions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	authTypes, diags2 := expandStringList(ctx, plan.AuthTypes)
	resp.Diagnostics.Append(diags2...)
	if resp.Diagnostics.HasError() {
		return
	}
	cacerts, diags3 := expandStringList(ctx, plan.CACertificates)
	resp.Diagnostics.Append(diags3...)
	if resp.Diagnostics.HasError() {
		return
	}

	// convert []string auth types to jujucloud.AuthTypes
	at := make(jujucloud.AuthTypes, len(authTypes))
	for i, s := range authTypes {
		at[i] = jujucloud.AuthType(s)
	}

	input := juju.UpdateCloudInput{
		Name:             plan.Name.ValueString(),
		Type:             plan.Type.ValueString(),
		Description:      "",
		AuthTypes:        at,
		Endpoint:         plan.Endpoint.ValueString(),
		IdentityEndpoint: plan.IdentityEndpoint.ValueString(),
		StorageEndpoint:  plan.StorageEndpoint.ValueString(),
		Regions:          regions,
		CACertificates:   cacerts,
	}
	if err := r.client.Clouds.UpdateCloud(input); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update cloud, got error %s", err))
		return
	}

	r.trace(fmt.Sprintf("Updated cloud %s", plan.Name.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the cloud from the controller.
func (r *cloudResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "cloud", "delete")
		return
	}
	var state cloudResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Clouds.RemoveCloud(juju.RemoveCloudInput{Name: state.Name.ValueString()}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove cloud, got error %s", err))
		return
	}
	r.trace(fmt.Sprintf("Removed cloud %s", state.Name.ValueString()))
}

func (r *cloudResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(r.subCtx, LogResourceCloud, msg, additionalFields...)
}

func expandStringList(ctx context.Context, l types.List) ([]string, diag.Diagnostics) {
	var result []string
	if l.IsNull() || l.IsUnknown() {
		return result, nil
	}

	return result, l.ElementsAs(ctx, &result, false)
}

func expandRegions(ctx context.Context, list types.List) ([]jujucloud.Region, diag.Diagnostics) {
	var diags diag.Diagnostics
	var regModels []cloudRegionModel
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	diags.Append(list.ElementsAs(ctx, &regModels, false)...)
	if diags.HasError() {
		return nil, diags
	}
	regions := make([]jujucloud.Region, 0, len(regModels))
	for _, rm := range regModels {
		regions = append(regions, jujucloud.Region{
			Name:             rm.Name.ValueString(),
			Endpoint:         rm.Endpoint.ValueString(),
			IdentityEndpoint: rm.IdentityEndpoint.ValueString(),
			StorageEndpoint:  rm.StorageEndpoint.ValueString(),
		})
	}
	return regions, diags
}

func flattenRegions(ctx context.Context, regions []jujucloud.Region) (types.List, diag.Diagnostics) {
	items := make([]cloudRegionModel, 0, len(regions))

	for _, r := range regions {
		items = append(items, cloudRegionModel{
			Name: types.StringValue(r.Name),
			Endpoint: func() types.String {
				if r.Endpoint == "" {
					return types.StringNull()
				}
				return types.StringValue(r.Endpoint)
			}(),
			IdentityEndpoint: func() types.String {
				if r.IdentityEndpoint == "" {
					return types.StringNull()
				}
				return types.StringValue(r.IdentityEndpoint)
			}(),
			StorageEndpoint: func() types.String {
				if r.StorageEndpoint == "" {
					return types.StringNull()
				}
				return types.StringValue(r.StorageEndpoint)
			}(),
		})
	}

	lst, diags := types.ListValueFrom(ctx, types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":              types.StringType,
			"endpoint":          types.StringType,
			"identity_endpoint": types.StringType,
			"storage_endpoint":  types.StringType,
		},
	}, items)

	return lst, diags
}

type defaultRegionForCloud struct{}

// DefaultList implements [defaults.List.DefaultList] for a default cloud region.
func (d defaultRegionForCloud) DefaultList(ctx context.Context, _ defaults.ListRequest, res *defaults.ListResponse) {
	elemType := types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"name":              types.StringType,
			"endpoint":          types.StringType,
			"identity_endpoint": types.StringType,
			"storage_endpoint":  types.StringType,
		},
	}

	obj, diags := types.ObjectValue(
		elemType.AttrTypes,
		map[string]attr.Value{
			"name":              types.StringValue(string(jujucloud.DefaultCloudRegion)),
			"endpoint":          types.StringNull(),
			"identity_endpoint": types.StringNull(),
			"storage_endpoint":  types.StringNull(),
		},
	)
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	list, diags := types.ListValue(elemType, []attr.Value{obj})
	res.Diagnostics.Append(diags...)
	if res.Diagnostics.HasError() {
		return
	}

	res.PlanValue = list
}

// Description implements [defaults.Describer.Description].
//
// Description should describe the default in plain text formatting.
// This information is used by provider logging and provider tooling such
// as documentation generation.
//
// The description should:
//   - Begin with a lowercase or other character suitable for the middle of
//     a sentence.
//   - End without punctuation.
func (d defaultRegionForCloud) Description(ctx context.Context) string {
	return "all clouds must have at least one default region and by default, the region named 'default' will be used"
}

// MarkdownDescription implements [defaults.Describer.MarkdownDescription].
//
// MarkdownDescription should describe the default in Markdown
// formatting. This information is used by provider logging and provider
// tooling such as documentation generation.
//
// The description should:
//   - Begin with a lowercase or other character suitable for the middle of
//     a sentence.
//   - End without punctuation.
func (d defaultRegionForCloud) MarkdownDescription(ctx context.Context) string {
	return "all clouds must have at least one default region and by default, the region named 'default' will be used"
}
