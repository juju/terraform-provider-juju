// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ resource.Resource = &storagePoolResource{}
var _ resource.ResourceWithConfigure = &storagePoolResource{}

// I don't think importing storage pools makes sense because we cannot also import the storage already produced from it with it.
// var _ resource.ResourceWithImportState = &storagePoolResource{}

// NewStoragePoolResource returns a new instance of the storage pool resource.
func NewStoragePoolResource() resource.Resource {
	return &storagePoolResource{}
}

type storagePoolResource struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

type storagePoolResourceModel struct {
	Name            types.String `tfsdk:"name"`
	Model           types.String `tfsdk:"model"`
	StorageProvider types.String `tfsdk:"storageprovider"`
	Attributes      types.Map    `tfsdk:"attributes"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (r *storagePoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.config = provider.Config
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceStoragePool)
}

func (r *storagePoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_pool"
}

func (r *storagePoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju storage pool.",
		Attributes: map[string]schema.Attribute{
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the storage pool.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"model": schema.StringAttribute{
				Description: "The name of the model where the storage pool will be created.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"storageprovider": schema.StringAttribute{
				Description: "The storage provider type (e.g., 'rootfs', 'tmpfs', 'loop', or a cloud specific provider).",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
					// Unfortunately, we cannot validate these, Juju has *_ProviderType in each environ and no defined data type
					// of all the available providers. We could hardcode it, but that would require us to maintain the list.
					// The simplest solution is to allow Juju to return an error if the provider is not valid.
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				Description: "Attributes for the storage pool.",
				ElementType: types.StringType,
				Optional:    true,
			},
		},
	}
}

func (r *storagePoolResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "storagepool", "create")
		return
	}

	var plan storagePoolResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Adhere to client on "any".
	var storageProviderAttrs map[string]string = make(map[string]string)
	resp.Diagnostics.Append(plan.Attributes.ElementsAs(ctx, &storageProviderAttrs, false)...)

	// Convert map[string]string to map[any]any
	storageProviderAttrsAny := make(map[string]any, len(storageProviderAttrs))
	for k, v := range storageProviderAttrs {
		storageProviderAttrsAny[k] = v
	}

	if err := r.client.Storage.CreatePool(
		plan.Model.ValueString(),
		plan.Name.ValueString(),
		plan.StorageProvider.ValueString(),
		storageProviderAttrsAny,
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create storage pool resource, got error: %s", err))
		return
	}
	r.trace("created storage pool", map[string]interface{}{
		"name":            plan.Name.ValueString(),
		"model":           plan.Model.ValueString(),
		"storageprovider": plan.StorageProvider.ValueString(),
	})

	plan.ID = types.StringValue(
		fmt.Sprintf("%s-%s", plan.Model.ValueString(), plan.Name.ValueString()),
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *storagePoolResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "storagepool", "read")
		return
	}

	var state storagePoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pool, err := r.client.Storage.GetPool(
		state.Model.ValueString(),
		state.StorageProvider.ValueString(),
		state.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read storage pool resource, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("read storage pool resource %q", pool.Name))

	convertedAttrs, diagErrs := types.MapValueFrom(ctx, types.StringType, pool.Attrs)
	resp.Diagnostics.Append(diagErrs...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.Name = types.StringValue(pool.Name)
	state.StorageProvider = types.StringValue(pool.Provider)
	state.Attributes = convertedAttrs

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *storagePoolResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "storagepool", "update")
		return
	}

	var plan, state storagePoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.Attributes.Equal(state.Attributes) {
		return
	}

	// Adhere to client on "any".
	var newStorageProviderAttrs map[string]any
	resp.Diagnostics.Append(plan.Attributes.ElementsAs(ctx, &newStorageProviderAttrs, false)...)

	if err := r.client.Storage.UpdatePool(
		state.Model.ValueString(),
		state.Name.ValueString(),
		state.StorageProvider.ValueString(),
		newStorageProviderAttrs,
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update storage pool resource, got error: %s", err))
		return
	}
	r.trace("updated storage pool", map[string]interface{}{
		"name":            plan.Name.ValueString(),
		"model":           plan.Model.ValueString(),
		"storageprovider": plan.StorageProvider.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *storagePoolResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "storagepool", "delete")
		return
	}

	var state storagePoolResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Storage.RemovePool(
		state.Model.ValueString(),
		state.Name.ValueString(),
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete storage pool resource, got error: %s", err))
		return
	}
	r.trace("deleted storage pool", map[string]interface{}{
		"name":            state.Name.ValueString(),
		"model":           state.Model.ValueString(),
		"storageprovider": state.StorageProvider.ValueString(),
	})
}

func (s *storagePoolResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(s.subCtx, LogResourceStoragePool, msg, additionalFields...)
}
