// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/attr"
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

type listMachinesRequest struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	Name      types.String `tfsdk:"name"`
}

type machineLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewMachineLister returns a new instance of the machine lister, which implements the ListResourceWithConfigure interface.
func NewMachineLister() list.ListResourceWithConfigure {
	return &machineLister{}
}

// Configure implements the ResourceWithConfigure interface, providing the provider data to the resource.
func (r *machineLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceMachine)
}

// Metadata returns the full name of the resource.
func (r *machineLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

// ListResourceConfigSchema implements the ListResourceSchema interface.
func (r *machineLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model in which to list machines.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the machine.",
				Optional:    true,
			},
		},
	}
}

// List implements the ListResource interface, retrieving the list of machines and sending them to the framework.
func (r *machineLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listMachinesRequest
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	machineIDs, err := r.client.Machines.ListMachines(listRequest.ModelUUID.ValueString())
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list machines in model %s, got error: %s", listRequest.ModelUUID.ValueString(), err),
				),
			},
		)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, machineID := range machineIDs {
			result := req.NewListResult(ctx)

			identity := machineResourceIdentityModel{
				ID: types.StringValue(
					newMachineID(
						listRequest.ModelUUID.ValueString(),
						machineID,
						listRequest.Name.ValueString(),
					),
				),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}
			if req.IncludeResource {
				machine, err := readMachine(ctx, r.client, listRequest.ModelUUID.ValueString(), machineID, false)
				if err != nil {
					result.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read machine %s, got error: %s", machineID, err))
					if !push(result) {
						return
					}
					continue
				}

				machine.Timeouts = timeouts.Value{
					Object: types.ObjectValueMust(
						map[string]attr.Type{
							"create": types.StringType,
						},
						map[string]attr.Value{
							"create": types.StringValue("30m"),
						},
					),
				}
				result.DisplayName = machine.MachineID.ValueString()
				result.Diagnostics.Append(result.Resource.Set(ctx, machine)...)
			}

			if !push(result) {
				return
			}
		}
	}
}
