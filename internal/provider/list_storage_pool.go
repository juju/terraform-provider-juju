// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type listStoragePoolRequest struct {
	ModelUUID       types.String `tfsdk:"model_uuid"`
	Name            types.String `tfsdk:"name"`
	StorageProvider types.String `tfsdk:"storage_provider"`
}

type storagePoolLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewStoragePoolLister returns a new instance of the storage pool lister.
func NewStoragePoolLister() list.ListResourceWithConfigure {
	return &storagePoolLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *storagePoolLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.config = provider.Config
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceStoragePool)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *storagePoolLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_storage_pool"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *storagePoolLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The Juju model UUID.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": schema.StringAttribute{
				Description: "Filter by storage pool name.",
				Optional:    true,
			},
			"storage_provider": schema.StringAttribute{
				Description: "Filter by storage provider type.",
				Optional:    true,
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *storagePoolLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listStoragePoolRequest

	// Read list config data into the model.
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()
	providers := []string{}
	names := []string{}

	if !listRequest.StorageProvider.IsNull() && !listRequest.StorageProvider.IsUnknown() {
		providers = append(providers, listRequest.StorageProvider.ValueString())
	}
	if !listRequest.Name.IsNull() && !listRequest.Name.IsUnknown() {
		names = append(names, listRequest.Name.ValueString())
	}

	pools, err := r.client.Storage.ListPools(ctx, juju.ListStoragePoolsInput{
		ModelUUID: modelUUID,
		Providers: providers,
		Names:     names,
	})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list storage pools in model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, pool := range pools {
			// Create result.
			result := req.NewListResult(ctx)

			// Set display name.
			result.DisplayName = pool.Pool.Name

			resourceModel := storagePoolResourceModel{
				Name:      types.StringValue(pool.Pool.Name),
				ModelUUID: types.StringValue(modelUUID),
			}

			identity := storagePoolResourceIdentityModel{
				ID: generateResourceID(resourceModel),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				storagePoolResource := storagePoolResourceModel{
					ID:              identity.ID,
					Name:            types.StringValue(pool.Pool.Name),
					ModelUUID:       types.StringValue(modelUUID),
					StorageProvider: types.StringValue(pool.Pool.Provider),
					Attributes:      types.MapNull(types.StringType),
				}

				if len(pool.Pool.Attrs) > 0 {
					convertedAttrs, dErr := types.MapValueFrom(ctx, types.StringType, pool.Pool.Attrs)
					result.Diagnostics.Append(dErr...)
					if result.Diagnostics.HasError() {
						push(result)
						return
					}
					storagePoolResource.Attributes = convertedAttrs
				}

				result.Diagnostics.Append(result.Resource.Set(ctx, storagePoolResource)...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}
			}

			if !push(result) {
				return
			}
		}
	}
}
