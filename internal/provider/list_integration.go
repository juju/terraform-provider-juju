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
	"github.com/juju/juju/core/crossmodel"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type listIntegrationRequest struct {
	ModelUUID       types.String `tfsdk:"model_uuid"`
	ApplicationName types.String `tfsdk:"application_name"`
}

type integrationLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewIntegrationLister returns a new instance of the integration lister.
func NewIntegrationLister() list.ListResourceWithConfigure {
	return &integrationLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *integrationLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceIntegration)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *integrationLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *integrationLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
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
				Description: "Filter integrations to those that include the given application name.",
				Optional:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidApplication, "must be a valid application name"),
				},
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *integrationLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listIntegrationRequest

	// Read list config data into the model
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()
	filterName := listRequest.ApplicationName.ValueString()

	integrations, err := r.client.Integrations.ListIntegrations(&juju.ListIntegrationsInput{
		ModelUUID: modelUUID,
	})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list integrations in model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, integration := range integrations {
			if filterName != "" && !integrationHasApplicationName(integration.Applications, filterName) {
				continue
			}

			result := req.NewListResult(ctx)
			identity := integrationResourceIdentityModel{
				ID: types.StringValue(newIDForIntegrationResource(modelUUID, integration.Applications)),
			}
			result.DisplayName = identity.ID.ValueString()
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				resourceSchema, ok := req.ResourceSchema.(schema.Schema)
				if !ok {
					result.Diagnostics.AddError(
						"Unexpected Resource Schema Type",
						fmt.Sprintf("Expected schema.Schema, got: %T. Please report this issue to the provider developers.", req.ResourceSchema),
					)
					push(result)
					return
				}

				resource, dErr := r.getIntegrationResource(ctx, resourceSchema, modelUUID, integration.Applications)
				if dErr.HasError() {
					result.Diagnostics.Append(dErr...)
					push(result)
					return
				}
				resource.ID = identity.ID

				result.Diagnostics.Append(result.Resource.Set(ctx, resource)...)
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

func (r *integrationLister) getIntegrationResource(
	ctx context.Context,
	resourceSchema schema.Schema,
	modelUUID string,
	apps []juju.Application,
) (integrationResourceModelV1, diag.Diagnostics) {
	applicationType := resourceSchema.GetBlocks()["application"].(schema.SetNestedBlock).NestedObject.Type()

	resource := integrationResourceModelV1{
		integrationResourceModel: integrationResourceModel{
			Via:         types.StringNull(),
			Application: types.SetNull(applicationType),
			ID:          types.StringNull(),
		},
		ModelUUID: types.StringValue(modelUUID),
	}

	applications, dErr := r.parseIntegrationApplications(apps)
	if dErr.HasError() {
		return integrationResourceModelV1{}, dErr
	}

	applicationSet, errDiag := types.SetValueFrom(ctx, applicationType, applications)
	dErr.Append(errDiag...)
	if dErr.HasError() {
		return integrationResourceModelV1{}, dErr
	}
	resource.Application = applicationSet

	return resource, dErr
}

func (r *integrationLister) parseIntegrationApplications(apps []juju.Application) ([]nestedApplication, diag.Diagnostics) {
	applications := make([]nestedApplication, len(apps))
	var diags diag.Diagnostics

	for i, app := range apps {
		application := nestedApplication{}
		if app.OfferURL != nil {
			url, err := crossmodel.ParseOfferURL(*app.OfferURL)
			if err != nil {
				diags.AddError("Client Error", fmt.Sprintf("Unable to parse offer URL %q: %s", *app.OfferURL, err))
				return nil, diags
			}
			application.OfferURL = types.StringValue(url.AsLocal().String())
			if r.client.Offers.IsOfferingController(url.Source) {
				application.OfferingController = types.StringValue(url.Source)
			}
			application.Endpoint = types.StringValue(app.Endpoint)
		} else {
			application.Endpoint = types.StringValue(app.Endpoint)
			application.Name = types.StringValue(app.Name)
		}
		applications[i] = application
	}

	return applications, diags
}

func integrationHasApplicationName(apps []juju.Application, name string) bool {
	for _, app := range apps {
		if app.Name == name {
			return true
		}
	}
	return false
}
