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
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type listSecretRequest struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	Name      types.String `tfsdk:"name"`
}

type secretLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewSecretLister returns a new instance of the secret lister.
func NewSecretLister() list.ListResourceWithConfigure {
	return &secretLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *secretLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSecret)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *secretLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *secretLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": listschema.StringAttribute{
				Description: "The Juju model UUID.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
			"name": listschema.StringAttribute{
				Description: "Filter by secret name.",
				Optional:    true,
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *secretLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listSecretRequest

	// Read list config data into the model
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()
	var secretName *string
	if !listRequest.Name.IsNull() && !listRequest.Name.IsUnknown() {
		value := listRequest.Name.ValueString()
		secretName = &value
	}

	secrets, err := r.client.Secrets.ListSecrets(&juju.ListSecretsInput{
		ModelUUID: modelUUID,
		Name:      secretName,
	})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list secrets in model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for _, secret := range secrets {
			// Create result.
			result := req.NewListResult(ctx)
			resourceID := types.StringValue(newSecretID(modelUUID, secret.SecretId))

			// Set display name.
			if secret.Name != "" {
				result.DisplayName = secret.Name
			} else {
				result.DisplayName = secret.SecretId
			}

			// Set identity.
			identity := secretResourceIdentityModel{
				ID: resourceID,
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				push(result)
				return
			}

			if req.IncludeResource {
				resource := secretResourceModelV1{
					ModelUUID: types.StringValue(modelUUID),
					secretResourceModel: secretResourceModel{
						SecretId:  types.StringValue(secret.SecretId),
						SecretURI: types.StringValue(secret.SecretURI),
						ID:        resourceID,
						Info:      types.StringNull(),
						Name:      types.StringNull(),
					},
				}

				if secret.Name != "" {
					resource.Name = types.StringValue(secret.Name)
				}
				if secret.Info != "" {
					resource.Info = types.StringValue(secret.Info)
				}

				secretValue, dErr := types.MapValueFrom(ctx, types.StringType, secret.Value)
				result.Diagnostics.Append(dErr...)
				if result.Diagnostics.HasError() {
					push(result)
					return
				}
				resource.Value = secretValue

				result.Diagnostics.Append(result.Resource.Set(ctx, resource)...)
				if result.Diagnostics.HasError() {
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
