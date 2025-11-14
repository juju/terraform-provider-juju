package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

type modelLister struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

func NewModelLister() list.ListResourceWithConfigure {
	return &modelLister{}
}

func (r *modelLister) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceModel)
}

func (r *modelLister) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelLister) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Attributes: map[string]listschema.Attribute{},
	}
}

func (r *modelLister) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	stream.Results = func(push func(list.ListResult) bool) {
		result := req.NewListResult(ctx)
		ids, err := r.client.Models.ListModels()
		if err != nil {
			result.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list models, got error: %s", err))
			return
		}
		for _, id := range ids {
			result.DisplayName = id
			identity := modelResourceIdentityModel{
				ID: types.StringValue(id),
			}
			result.Diagnostics.Append(result.Identity.Set(ctx, identity)...)
			if result.Diagnostics.HasError() {
				return
			}
			if req.IncludeResource {
				schema, ok := req.ResourceSchema.(schema.Schema)
				if !ok {
					result.Diagnostics.AddError(
						"Unexpected Resource Schema Type",
						fmt.Sprintf("Expected schema.Schema, got: %T. Please report this issue to the provider developers.", req.ResourceSchema),
					)
					return
				}
				resource, err := r.GetModelResource(ctx, id, schema)
				if err.HasError() {
					result.Diagnostics.Append(err...)
					return
				}
				result.Diagnostics.Append(result.Resource.Set(ctx, resource)...)
				if result.Diagnostics.HasError() {
					return
				}
			}
			if !push(result) {
				return
			}
		}
	}
}

func (r *modelLister) GetModelResource(ctx context.Context, modelUUID string, sc schema.Schema) (*modelResourceModel, diag.Diagnostics) {
	resource := &modelResourceModel{}
	diags := diag.Diagnostics{}
	response, err := r.client.Models.ReadModel(modelUUID)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to read model with UUID %s, got error: %s", modelUUID, err))
		return nil, diags
	}
	// Acquire cloud, credential, and config
	tag, err := names.ParseCloudCredentialTag(response.ModelInfo.CloudCredentialTag)
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to parse cloud credential tag for model, got error: %s", err))
		return nil, diags
	}
	credential := tag.Name()

	// Set the read values into the new state model
	// Cloud
	if response.ModelInfo.CloudTag != "" && response.ModelInfo.CloudRegion != "" {
		cloudList := []nestedCloud{{
			Name:   types.StringValue(strings.TrimPrefix(response.ModelInfo.CloudTag, juju.PrefixCloud)),
			Region: types.StringValue(response.ModelInfo.CloudRegion),
		}}
		cloudType := sc.GetBlocks()["cloud"].(schema.ListNestedBlock).NestedObject.Type()
		newStateCloud, errDiag := types.ListValueFrom(ctx, cloudType, cloudList)
		diags.Append(errDiag...)
		if diags.HasError() {
			return nil, diags
		}
		resource.Cloud = newStateCloud
	}

	// Constraints
	if response.ModelConstraints.String() != "" {
		resource.Constraints = types.StringValue(response.ModelConstraints.String())
	}

	// Config
	if len(response.ModelConfig) > 0 {
		config, diags := newConfigFromModelConfigAPI(ctx, response.ModelConfig, types.MapNull(types.StringType))
		diags.Append(diags...)
		if diags.HasError() {
			return nil, diags
		}
		resource.Config, diags = types.MapValueFrom(ctx, types.StringType, config)
		diags.Append(diags...)
		if diags.HasError() {
			return nil, diags
		}
	}

	annotations, err := r.client.Annotations.GetAnnotations(&juju.GetAnnotationsInput{
		EntityTag: names.NewModelTag(response.ModelInfo.UUID),
		ModelUUID: modelUUID,
	})
	if err != nil {
		diags.AddError("Client Error", fmt.Sprintf("Unable to get model's annotations, got error: %s", err))
		return nil, diags
	}
	if len(annotations.Annotations) > 0 {
		annotationsType := sc.GetAttributes()["annotations"].(schema.MapAttribute).ElementType

		annotationsMapValue, errDiag := types.MapValueFrom(ctx, annotationsType, annotations.Annotations)
		diags.Append(errDiag...)
		if diags.HasError() {
			return nil, diags
		}
		resource.Annotations = annotationsMapValue
	} else {
		resource.Annotations = types.MapNull(types.StringType)
	}

	resource.Name = types.StringValue(response.ModelInfo.Name)
	resource.Type = types.StringValue(response.ModelInfo.Type)
	resource.Credential = types.StringValue(credential)
	resource.UUID = types.StringValue(response.ModelInfo.UUID)
	resource.ID = types.StringValue(response.ModelInfo.UUID)

	return resource, diags
}
