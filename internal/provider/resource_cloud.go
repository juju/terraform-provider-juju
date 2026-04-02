// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

// NewCloudResource returns a cloud resource.
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
	AuthTypes        types.Set    `tfsdk:"auth_types"`
	Endpoint         types.String `tfsdk:"endpoint"`
	IdentityEndpoint types.String `tfsdk:"identity_endpoint"`
	StorageEndpoint  types.String `tfsdk:"storage_endpoint"`
	CACertificates   types.Set    `tfsdk:"ca_certificates"`
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
				Description: "The name of the cloud in Juju.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Description: "The type of the cloud.",
				Required:    true,
			},
			"auth_types": schema.SetAttribute{
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
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						replaceIfIdentityOrStorageEndpointUnset,
						"identity_endpoint cannot be unset once set (resource must be replaced)",
						"identity_endpoint cannot be unset once set (resource must be replaced)",
					),
				},
			},
			"storage_endpoint": schema.StringAttribute{
				Description: "Optional global storage endpoint for the cloud.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(
						replaceIfIdentityOrStorageEndpointUnset,
						"storage_endpoint cannot be unset once set (resource must be replaced)",
						"storage_endpoint cannot be unset once set (resource must be replaced)",
					),
				},
			},
			"ca_certificates": schema.SetAttribute{
				Description: "List of PEM-encoded X509 certificates for the cloud.",
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
				// Juju doesn't validate the certificates on add/update, but we can at least
				// ensure they are valid PEM-encoded certs here.
				Validators: []validator.Set{ValidateCACertificatesPEM()},
			},
			// All clouds must have at least one default region. Juju has a default region named "default" that is used
			// if no regions are specified. This is provided by the CLI client when adding clouds without regions.
			// As such we are copying that behaviour here by providing a default region named "default" if no regions are specified.
			"regions": schema.ListNestedAttribute{
				Description: "List of regions for the cloud. The first region in the list is the default region for the cloud.",
				Computed:    true,
				Optional:    true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Required: true, Description: "Name of the region."},
						"endpoint": schema.StringAttribute{
							Optional:    true,
							Description: "Region-specific endpoint.",
							Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						},
						"identity_endpoint": schema.StringAttribute{
							Optional:    true,
							Description: "Region-specific identity endpoint.",
							Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						},
						"storage_endpoint": schema.StringAttribute{
							Optional:    true,
							Description: "Region-specific storage endpoint.",
							Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
						},
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

	regions := expandRegions(ctx, plan.Regions, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// This shouldn't happen due to the default, but just in case.
	if len(regions) == 0 {
		resp.Diagnostics.AddError("Plan Error", "Field `regions` must contain at least one region.")
		return
	}

	authTypes := expandStringList(ctx, plan.AuthTypes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	cacerts := expandStringList(ctx, plan.CACertificates, &resp.Diagnostics)
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
	state.AuthTypes, dErr = types.SetValueFrom(ctx, types.StringType, out.AuthTypes)
	if dErr.HasError() {
		resp.Diagnostics.Append(dErr...)
		return
	}

	// Normalize optional global endpoints: Juju may return empty strings; we store null to avoid drift.
	// We can't do this right now though as when trying to set IdentityEndpoint or StorageEndpoint to "", Juju isn't
	// returning an error, but instead the previous values they were set to.
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

	// Regions comes back in an ordered lexicographical list from Juju, but we
	// want it to match the ordering we already have in state
	// (users may rely on index 0 being the default region).
	// We do this as we use the 1st region in the List as the default.
	orderedRegions := make([]jujucloud.Region, 0, len(out.Regions))

	// Build a lookup of regions returned by Juju
	outRegionByName := make(map[string]jujucloud.Region, len(out.Regions))
	for _, rg := range out.Regions {
		outRegionByName[rg.Name] = rg
	}

	// Read regions from state to preserve ordering
	var stateCrms []cloudRegionModel
	resp.Diagnostics.Append(state.Regions.ElementsAs(ctx, &stateCrms, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// First, add regions that exist in state, in state order,
	// and remove them from the lookup map as they are consumed.
	for _, crm := range stateCrms {
		if rg, ok := outRegionByName[crm.Name.ValueString()]; ok {
			orderedRegions = append(orderedRegions, rg)
			delete(outRegionByName, crm.Name.ValueString())
		}
	}

	// Append any regions that exist in Juju but were not present in state
	// (e.g. added out-of-band). Iterate Juju's original slice so that newly
	// discovered regions are appended in a deterministic (lexicographical)
	// order, AFTER all state-ordered regions.
	for _, rg := range out.Regions {
		if _, stillMissing := outRegionByName[rg.Name]; stillMissing {
			orderedRegions = append(orderedRegions, rg)
		}
	}

	lst := flattenRegions(ctx, orderedRegions, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Regions = lst

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

	regions := expandRegions(ctx, plan.Regions, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	authTypes := expandStringList(ctx, plan.AuthTypes, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	cacerts := expandStringList(ctx, plan.CACertificates, &resp.Diagnostics)
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

func expandStringList(ctx context.Context, s types.Set, resp *diag.Diagnostics) []string {
	var result []string

	resp.Append(s.ElementsAs(ctx, &result, false)...)

	return result
}

func expandRegions(ctx context.Context, list types.List, resp *diag.Diagnostics) []jujucloud.Region {
	var regModels []cloudRegionModel

	resp.Append(list.ElementsAs(ctx, &regModels, false)...)

	regions := make([]jujucloud.Region, 0, len(regModels))
	for _, rm := range regModels {
		regions = append(regions, jujucloud.Region{
			Name:             rm.Name.ValueString(),
			Endpoint:         rm.Endpoint.ValueString(),
			IdentityEndpoint: rm.IdentityEndpoint.ValueString(),
			StorageEndpoint:  rm.StorageEndpoint.ValueString(),
		})
	}
	return regions
}

func flattenRegions(ctx context.Context, regions []jujucloud.Region, resp *diag.Diagnostics) types.List {
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

	resp.Append(diags...)

	return lst
}

// replaceIfIdentityOrStorageEndpointUnset is a plan modifier function that forces replacement
// of the attribute if it was set, and the user unsets it by removing the field (plan becomes null).
// They cannot set it to "" due to validation.
func replaceIfIdentityOrStorageEndpointUnset(ctx context.Context, sr planmodifier.StringRequest, rrifr *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	// Force replacement if the attribute was set, and the user unsets it by removing
	// the field (plan becomes null). They cannot set it to "" due to validation.
	if sr.StateValue.IsNull() || sr.StateValue.IsUnknown() {
		return
	}
	if sr.PlanValue.IsUnknown() {
		return
	}
	if sr.PlanValue.IsNull() {
		rrifr.RequiresReplace = true
	}
}

// defaultRegionForCloud implements a default for the regions attribute of the cloud resource.
// It is a list with a single region element, where it's name is [jujucloud.DefaultCloudRegion].
type defaultRegionForCloud struct{}

// DefaultSet implements [defaults.List.DefaultList] for a default cloud region.
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
