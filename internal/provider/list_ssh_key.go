// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/utils/v3/ssh"
)

type listSSHKeyRequest struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
}

type sshKeyLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

// NewSSHKeyLister returns a new instance of the SSH key lister.
func NewSSHKeyLister() list.ListResourceWithConfigure {
	return &sshKeyLister{}
}

// Configure implements [list.ListResourceWithConfigure].
func (r *sshKeyLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSSHKey)
}

// Metadata implements [list.ListResourceWithConfigure].
func (r *sshKeyLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ssh_key"
}

// ListResourceConfigSchema implements [list.ListResourceWithConfigure].
func (r *sshKeyLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The Juju model UUID.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
			},
		},
	}
}

// List implements [list.ListResourceWithConfigure].
func (r *sshKeyLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	var listRequest listSSHKeyRequest

	// Read list config data into the model
	diags := req.Config.Get(ctx, &listRequest)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	modelUUID := listRequest.ModelUUID.ValueString()

	keys, err := r.client.SSHKeys.ListKeys(juju.ListSSHKeysInput{
		Username:  r.client.Username(),
		ModelUUID: modelUUID,
	})
	if err != nil {
		stream.Results = list.ListResultsStreamDiagnostics(
			diag.Diagnostics{
				diag.NewErrorDiagnostic(
					"Client Error",
					fmt.Sprintf("Unable to list ssh keys in model %s, got error: %s", modelUUID, err),
				),
			},
		)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		for i, key := range keys {
			// Create result.
			result := req.NewListResult(ctx)

			// Set display name.
			// We don't really have anything "human friendly readable" to display for the key,
			// so we just display the index of the key in the list, regardless if it is ordered or not.
			kid := fmt.Sprintf("Key %d", i)
			result.DisplayName = kid

			// Set identity.
			fingerprint, _, err := ssh.KeyFingerprint(
				strings.TrimSuffix(key, "\n"),
			)
			if err != nil {
				result.Diagnostics.AddError("Malformed SSH Key", fmt.Sprintf("Unable to parse SSH key payload: %s", err))
				return
			}
			identity := sshKeyResourceIdentityModel{
				ID: types.StringValue(newSSHKeyID(modelUUID, fingerprint)),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)

			if req.IncludeResource {
				// Set resource.
				sshKeyResourceModelV1 := sshKeyResourceModelV1{
					ModelUUID: types.StringValue(modelUUID),
					sshKeyResourceModel: sshKeyResourceModel{
						Payload: types.StringValue(key),
					},
				}
				result.Diagnostics.Append(result.Resource.Set(ctx, sshKeyResourceModelV1)...)
			}

			// Send the result to the stream.
			if !push(result) {
				return
			}
		}
	}
}
