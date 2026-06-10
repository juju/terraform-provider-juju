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

type listSpaceRequest struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	Name      types.String `tfsdk:"name"`
}

type spaceLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewSpaceLister returns a new instance of the space lister.
func NewSpaceLister() list.ListResourceWithConfigure {
	return &spaceLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *spaceLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSpace)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *spaceLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *spaceLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
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
				Description: "Filter by space name.",
				Optional:    true,
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *spaceLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listSpaceRequest

	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()
	spaces, err := r.client.Spaces.ListSpaces(ctx, &juju.ListSpacesInput{ModelUUID: modelUUID})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list spaces in model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	filterByName := ""
	if !listRequest.Name.IsNull() && !listRequest.Name.IsUnknown() {
		filterByName = listRequest.Name.ValueString()
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, space := range spaces {
			if filterByName != "" && space.Name != filterByName {
				continue
			}

			result := req.NewListResult(ctx)
			result.DisplayName = space.Name

			identity := spaceResourceIdentityModel{
				ID: types.StringValue(newSpaceResourceID(modelUUID, space.Name)),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				spaceResource := spaceResourceModel{
					ID:        identity.ID,
					ModelUUID: types.StringValue(modelUUID),
					Name:      types.StringValue(space.Name),
				}

				result.Diagnostics.Append(result.Resource.Set(ctx, spaceResource)...)
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
