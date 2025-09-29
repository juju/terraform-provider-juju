// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"maps"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

var _ resource.Resource = &storagePoolResource{}
var _ resource.ResourceWithConfigure = &storagePoolResource{}

// NewStoragePoolResource returns a new instance of the storage pool resource.
func NewStoragePoolResource() resource.Resource {
	return &storagePoolResource{}
}

type storagePoolWaitForInput struct {
	modelname string
	pname     string
}

type storagePoolResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type storagePoolResourceModel struct {
	Name            types.String `tfsdk:"name"`
	ModelUUID       types.String `tfsdk:"model_uuid"`
	StorageProvider types.String `tfsdk:"storage_provider"`
	Attributes      types.Map    `tfsdk:"attributes"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Configure implements resource.ResourceWithConfigure.
func (r *storagePoolResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*juju.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *juju.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceStoragePool)
}

// Metadata implements resource.Resource.
func (r *storagePoolResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_pool"
}

// Schema implements resource.Resource.
func (r *storagePoolResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `A resource that represents a Juju storage pool.
		
To learn more about storage pools, please visit: https://documentation.ubuntu.com/juju/3.6/reference/storage/#storage-pool
		`,
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
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the storage pool will be created.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"storage_provider": schema.StringAttribute{
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

// Create implements resource.Resource.
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

	storageProviderAttrsAny, err := plan.getAttributesAsGoMap(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get storage provider attributes as map, got error: %s", err))
		return
	}

	if err := r.client.Storage.CreatePool(
		juju.CreateStoragePoolInput{
			ModelUUID: plan.ModelUUID.ValueString(),
			PoolName:  plan.Name.ValueString(),
			Provider:  plan.StorageProvider.ValueString(),
			Attrs:     storageProviderAttrsAny,
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create storage pool resource, got error: %s", err))
		return
	}

	// Wait for the pool to be created.
	if _, err := wait.WaitFor(
		wait.WaitForCfg[storagePoolWaitForInput, juju.GetStoragePoolResponse]{
			Context: ctx,
			GetData: func(input storagePoolWaitForInput) (juju.GetStoragePoolResponse, error) {
				return r.client.Storage.GetPool(
					juju.GetStoragePoolInput{
						ModelUUID: input.modelname,
						PoolName:  input.pname,
					},
				)
			},
			Input: storagePoolWaitForInput{
				modelname: plan.ModelUUID.ValueString(),
				pname:     plan.Name.ValueString(),
			},
			DataAssertions: []wait.Assert[juju.GetStoragePoolResponse]{
				func(data juju.GetStoragePoolResponse) error {
					if data.Pool.Name != plan.Name.ValueString() {
						return juju.NewRetryReadError("waiting for storage pool to be created")
					}
					return nil
				},
			},
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for storage pool, got error: %s", err))
		return
	}

	r.trace("created storage pool", map[string]interface{}{
		"name":             plan.Name.ValueString(),
		"model":            plan.ModelUUID.ValueString(),
		"storage_provider": plan.StorageProvider.ValueString(),
	})

	plan.ID = generateResourceID(plan)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read implements resource.Resource.
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

	getPoolResp, err := r.client.Storage.GetPool(
		juju.GetStoragePoolInput{
			ModelUUID: state.ModelUUID.ValueString(),
			PoolName:  state.Name.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read storage pool resource, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("read storage pool resource %q", getPoolResp.Pool.Name))

	// If user removes the attribute block entirely during an update, we want to maintain the null value in state.
	// So, if the map has values, we set it, otherwise we leave it as null.
	if len(getPoolResp.Pool.Attrs) > 0 {
		convertedAttrs, diagErrs := types.MapValueFrom(ctx, types.StringType, getPoolResp.Pool.Attrs)
		resp.Diagnostics.Append(diagErrs...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Attributes = convertedAttrs
	}

	state.Name = types.StringValue(getPoolResp.Pool.Name)
	state.StorageProvider = types.StringValue(getPoolResp.Pool.Provider)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update implements resource.Resource.
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

	newStorageProviderAttrsAny, err := plan.getAttributesAsGoMap(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get storage provider attributes as map, got error: %s", err))
		return
	}

	if err := r.client.Storage.UpdatePool(
		state.ModelUUID.ValueString(),
		state.Name.ValueString(),
		state.StorageProvider.ValueString(),
		newStorageProviderAttrsAny,
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update storage pool resource, got error: %s", err))
		return
	}

	// Wait for the pool to be updated.
	if _, err := wait.WaitFor(
		wait.WaitForCfg[storagePoolWaitForInput, juju.GetStoragePoolResponse]{
			Context: ctx,
			GetData: func(input storagePoolWaitForInput) (juju.GetStoragePoolResponse, error) {
				return r.client.Storage.GetPool(
					juju.GetStoragePoolInput{
						ModelUUID: input.modelname,
						PoolName:  input.pname,
					},
				)
			},
			Input: storagePoolWaitForInput{
				modelname: plan.ModelUUID.ValueString(),
				pname:     plan.Name.ValueString(),
			},
			DataAssertions: []wait.Assert[juju.GetStoragePoolResponse]{
				func(data juju.GetStoragePoolResponse) error {
					// If the attributes are removed, or {}, Juju always returns an empty map, so we check for an empty map.
					if plan.Attributes.IsNull() || plan.Attributes.IsUnknown() || len(plan.Attributes.Elements()) == 0 {
						if len(data.Pool.Attrs) == 0 {
							return nil
						}
						return juju.NewRetryReadError("waiting for storage pool attributes to be removed")
					}

					planAttributesMapAny, err := plan.getAttributesAsGoMap(ctx)
					if err != nil {
						return err
					}

					// Attributes cannot be nested, so a simple maps.Equal is sufficient.
					if !maps.Equal(planAttributesMapAny, data.Pool.Attrs) {
						return juju.NewRetryReadError("waiting for storage pool attributes to be updated")
					}

					return nil
				},
			},
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for storage pool, got error: %s", err))
		return
	}

	r.trace("updated storage pool", map[string]interface{}{
		"name":             plan.Name.ValueString(),
		"model":            plan.ModelUUID.ValueString(),
		"storage_provider": plan.StorageProvider.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete implements resource.Resource.
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
		juju.RemoveStoragePoolInput{
			ModelUUID: state.ModelUUID.ValueString(),
			PoolName:  state.Name.ValueString(),
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete storage pool resource, got error: %s", err))
		return
	}

	// Wait for the pool to be deleted.
	if err := wait.WaitForError(
		wait.WaitForErrorCfg[storagePoolWaitForInput, juju.GetStoragePoolResponse]{
			Context: ctx,
			Input: storagePoolWaitForInput{
				modelname: state.ModelUUID.ValueString(),
				pname:     state.Name.ValueString(),
			},
			GetData: func(input storagePoolWaitForInput) (juju.GetStoragePoolResponse, error) {
				return r.client.Storage.GetPool(
					juju.GetStoragePoolInput{
						ModelUUID: input.modelname,
						PoolName:  input.pname,
					},
				)
			},
			ErrorToWait: juju.StoragePoolNotFoundError,
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for storage pool, got error: %s", err))
		return
	}
	r.trace("deleted storage pool", map[string]interface{}{
		"name":             state.Name.ValueString(),
		"model":            state.ModelUUID.ValueString(),
		"storage_provider": state.StorageProvider.ValueString(),
	})
}

func (s *storagePoolResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(s.subCtx, LogResourceStoragePool, msg, additionalFields...)
}

// getAttributesAsGoMap converts the attributes from the state model into a map[string]any and returns
// them for comparison.
func (s storagePoolResourceModel) getAttributesAsGoMap(ctx context.Context) (map[string]any, error) {
	casted := make(map[string]string)
	diags := s.Attributes.ElementsAs(ctx, &casted, false)
	if diags.HasError() {
		return nil, fmt.Errorf("failed to convert plan attributes to map for comparison in Update WaitFor: %v", diags)
	}

	planAttributesMapAny := make(map[string]any, len(casted))
	for k, v := range casted {
		planAttributesMapAny[k] = v
	}

	return planAttributesMapAny, nil
}

func generateResourceID(plan storagePoolResourceModel) basetypes.StringValue {
	return types.StringValue(
		fmt.Sprintf("%s-%s", plan.ModelUUID.ValueString(), plan.Name.ValueString()),
	)
}
