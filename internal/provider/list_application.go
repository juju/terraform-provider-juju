// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"slices"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/api/base"
	jujustorage "github.com/juju/juju/storage"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type listApplicationRequest struct {
	ModelUUID       types.String `tfsdk:"model_uuid"`
	ApplicationName types.String `tfsdk:"application_name"`
}

type applicationLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewApplicationLister returns a new instance of the application lister.
func NewApplicationLister() list.ListResourceWithConfigure {
	return &applicationLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *applicationLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceApplication)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *applicationLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_application"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *applicationLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The Juju model UUID.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"application_name": schema.StringAttribute{
				Description: "The Juju application name.",
				Optional:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidApplication, "must be a valid application name"),
				},
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *applicationLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listApplicationRequest

	// Read list config data into the model
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()

	status, err := r.client.Models.ReadModelStatus(modelUUID)
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to read model status for model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	// Extract the application names.
	appNames := make([]string, 0, len(status.ModelStatus.Applications))
	//
	if listRequest.ApplicationName.ValueString() != "" {
		i := slices.IndexFunc(status.ModelStatus.Applications, func(a base.Application) bool {
			return a.Name == listRequest.ApplicationName.ValueString()
		})
		if i != -1 {
			appNames = append(appNames, status.ModelStatus.Applications[i].Name)
		}
	} else {
		for _, app := range status.ModelStatus.Applications {
			appNames = append(appNames, app.Name)
		}
		sort.Strings(appNames)
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, appName := range appNames {
			// Create result.
			result := req.NewListResult(ctx)

			// Set display name.
			result.DisplayName = appName

			// Set identity.
			identity := applicationResourceIdentityModel{
				ID: types.StringValue(newAppID(modelUUID, appName)),
			}

			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				schema, ok := req.ResourceSchema.(schema.Schema)
				if !ok {
					result.Diagnostics.AddError(
						"Unexpected Resource Schema Type",
						fmt.Sprintf("Expected schema.Schema, got: %T. Please report this issue to the provider developers.", req.ResourceSchema),
					)
					push(result)
					return
				}

				resource, dErr := r.getApplicationResource(ctx, schema, modelUUID, appName)
				if dErr.HasError() {
					result.Diagnostics.Append(dErr...)
					push(result)
					return
				}

				resource.ID = identity.ID

				result.Diagnostics.Append(result.Resource.Set(ctx, resource)...)
				if result.Diagnostics.HasError() {
					result.Diagnostics.Append(dErr...)
					push(result)
					return
				}
			}

			// Send the result to the stream.
			if !push(result) {
				return
			}
		}
	}
}

func (r *applicationLister) getApplicationResource(
	ctx context.Context,
	resourceSchema schema.Schema,
	modelUUID string,
	appName string,
) (applicationResourceModelV1, diag.Diagnostics) {
	charmType := resourceSchema.GetBlocks()[CharmKey].(schema.ListNestedBlock).NestedObject.Type()
	endpointBindingsType := resourceSchema.GetAttributes()[EndpointBindingsKey].(schema.SetNestedAttribute).NestedObject.Type()
	exposeType := resourceSchema.GetBlocks()[ExposeKey].(schema.ListNestedBlock).NestedObject.Type()
	resourceType := resourceSchema.GetAttributes()[ResourceKey].(schema.MapAttribute).ElementType
	storageType := resourceSchema.GetAttributes()[StorageKey].(schema.SetNestedAttribute).NestedObject.Type()

	appModel := applicationResourceModelV1{
		applicationResourceModel: applicationResourceModel{
			Config:            types.MapNull(types.StringType),
			EndpointBindings:  types.SetNull(endpointBindingsType),
			Expose:            types.ListNull(exposeType),
			Machines:          types.SetNull(types.StringType),
			Resources:         types.MapNull(resourceType),
			StorageDirectives: types.MapNull(types.StringType),
			Storage:           types.SetNull(storageType),
			ID:                types.StringNull(),
		},
		ModelUUID: types.StringNull(),
	}

	diags := diag.Diagnostics{}

	// Read da app
	res, err := r.client.Applications.ReadApplication(&juju.ReadApplicationInput{
		ModelUUID: modelUUID,
		AppName:   appName,
	})
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to read application with name %s for model %s, got error: %s", modelUUID, appName, err))
		return applicationResourceModelV1{}, diags
	}
	if res == nil {
		diags.AddError("Client Error", "Read application response is nil")
		return applicationResourceModelV1{}, diags
	}

	// Set model UUID
	appModel.ModelUUID = types.StringValue(modelUUID)

	// Set app name
	appModel.ApplicationName = types.StringValue(appName)

	// Set charm
	dataCharm := nestedCharm{
		Name:     types.StringValue(res.Name),
		Channel:  types.StringValue(res.Channel),
		Revision: types.Int64Value(int64(res.Revision)),
		Base:     types.StringValue(res.Base),
	}
	charm, dErr := types.ListValueFrom(ctx, charmType, []nestedCharm{dataCharm})
	if dErr.HasError() {
		diags.Append(dErr...)
		return applicationResourceModelV1{}, diags
	}
	appModel.Charm = charm

	// Set config.
	if len(res.Config) > 0 {
		config, dErr := newConfigFromApplicationAPI(ctx, res.Config, appModel.Config)
		diags.Append(dErr...)
		if diags.HasError() {
			return applicationResourceModelV1{}, diags
		}

		appModel.Config, dErr = types.MapValueFrom(ctx, types.StringType, config)
		diags.Append(dErr...)
		if diags.HasError() {
			return applicationResourceModelV1{}, diags
		}
	}

	// Set constraints
	appModel.Constraints = NewCustomConstraintsValue(res.Constraints.String())

	// Set endpoint bindings
	if len(res.EndpointBindings) > 0 {
		appModel.EndpointBindings, dErr = toEndpointBindingsSet(ctx, endpointBindingsType, res.EndpointBindings)
		if dErr.HasError() {
			diags.Append(dErr...)
			return applicationResourceModelV1{}, diags
		}
	}

	// Set expose
	if res.Expose != nil {
		exp := parseNestedExpose(res.Expose)
		appModel.Expose, dErr = types.ListValueFrom(ctx, exposeType, []nestedExpose{exp})
		if dErr.HasError() {
			diags.Append(dErr...)
			return applicationResourceModelV1{}, diags
		}
	}

	// Set machines
	if len(res.Machines) > 0 {
		machines, dErr := types.SetValueFrom(ctx, types.StringType, res.Machines)
		if dErr.HasError() {
			diags.Append(dErr...)
			return applicationResourceModelV1{}, diags
		}
		appModel.Machines = machines
	}

	// Set model type
	appModel.ModelType = types.StringValue(res.ModelType)

	// Set resources
	if len(res.Resources) > 0 {
		appModel.Resources, dErr = types.MapValueFrom(ctx, resourceType, res.Resources)
		if dErr.HasError() {
			diags.Append(dErr...)
			return applicationResourceModelV1{}, diags
		}
	}

	// Set storage directives
	// This is a bit more involved as we need to convert map back into string form.
	stringifiedStorageDirectiveMap := make(map[string]string, len(res.Storage))
	for k, v := range res.Storage {
		directive, err := jujustorage.ToString(v)
		if err != nil {
			// Just because one has failed, we can still continue to list but throw the warning.
			diags.AddWarning("Parsing Error", fmt.Sprintf("Failed to parse storage constraint for key %s: %s", k, err.Error()))
		}
		stringifiedStorageDirectiveMap[k] = directive
	}
	appModel.StorageDirectives, dErr = types.MapValueFrom(ctx, types.StringType, stringifiedStorageDirectiveMap)
	if dErr.HasError() {
		diags.Append(dErr...)
		return applicationResourceModelV1{}, diags
	}
	// SetTrust
	appModel.Trust = types.BoolValue(res.Trust)

	// Set units
	if res.Principal || res.Units > 0 {
		appModel.UnitCount = types.Int64Value(int64(res.Units))
	} else {
		appModel.UnitCount = types.Int64Value(1)
	}

	return appModel, nil
}
