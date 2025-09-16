// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/clock"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/names/v5"
	"github.com/juju/utils/v3"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/retry"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

const (
	// maxModelDestroyWait provides a reasonably long wait time for a
	// model to be destroyed considering that we wait for all the
	// model's resources to be destroyed before starting the model deletion.
	maxModelDestroyWait = 1 * time.Minute
)

var _ resource.Resource = &modelResource{}
var _ resource.ResourceWithConfigure = &modelResource{}
var _ resource.ResourceWithImportState = &modelResource{}

func NewModelResource() resource.Resource {
	return &modelResource{}
}

type modelResource struct {
	client *juju.Client
	config juju.Config

	// context for the logging subsystem.
	subCtx context.Context
}

type modelResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Cloud       types.List   `tfsdk:"cloud"`
	Config      types.Map    `tfsdk:"config"`
	Constraints types.String `tfsdk:"constraints"`
	Annotations types.Map    `tfsdk:"annotations"`
	Credential  types.String `tfsdk:"credential"`
	Type        types.String `tfsdk:"type"`
	UUID        types.String `tfsdk:"uuid"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// nestedCloud represents an element in a Cloud list of a model resource
type nestedCloud struct {
	Name   types.String `tfsdk:"name"`
	Region types.String `tfsdk:"region"`
}

func (r *modelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Model.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name to be assigned to the model. Changing this value will" +
					" require the model to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uuid": schema.StringAttribute{
				Description: "The uuid of the model",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"config": schema.MapAttribute{
				Description: "Override default model configuration",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"annotations": schema.MapAttribute{
				Description: "Annotations for the model",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"constraints": schema.StringAttribute{
				Description: "Constraints imposed to this model",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credential": schema.StringAttribute{
				Description: "Credential used to add the model",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"type": schema.StringAttribute{
				Description: "Type of the model. Set by the Juju's API server",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"cloud": schema.ListNestedBlock{
				Description: "Juju Cloud where the model will operate. Changing this value will cause the" +
					" model to be destroyed and recreated by terraform.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the cloud",
							Required:    true,
						},
						"region": schema.StringAttribute{
							Description: "The region of the cloud",
							Optional:    true,
							Computed:    true,
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
					},
				},
			},
		},
	}
}

func (r *modelResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *modelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *modelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "model", "create")
		return
	}

	var plan modelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Acquire modelName, clouds, config, credential & constraints from the model plan
	modelName := plan.Name.ValueString()
	var clouds []nestedCloud
	resp.Diagnostics.Append(plan.Cloud.ElementsAs(ctx, &clouds, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config map[string]string
	var diags diag.Diagnostics
	config, diags = newConfig(ctx, plan.Config)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	credential := plan.Credential.ValueString()
	readConstraints := plan.Constraints.ValueString()

	parsedConstraints := constraints.Value{}
	var err error
	if readConstraints != "" {
		// TODO (cderici): this may be moved into internal/model so
		// resource_model can avoid importing juju/core/constraints
		parsedConstraints, err = constraints.Parse(readConstraints)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse constraints, got error: %s", err))
			return
		}
	}

	cloudNameInput, cloudRegionInput := "", ""

	if len(clouds) > 0 {
		cloudNameInput = clouds[0].Name.ValueString()
		cloudRegionInput = clouds[0].Region.ValueString()
	}

	response, err := retry.RetryOnErrors(retry.RetryOnErrorsCfg[juju.CreateModelInput, juju.CreateModelResponse]{
		Context: ctx,
		Input: juju.CreateModelInput{
			Name:        modelName,
			CloudName:   cloudNameInput,
			CloudRegion: cloudRegionInput,
			Config:      config,
			Constraints: parsedConstraints,
			Credential:  credential,
		},
		Do:              r.client.Models.CreateModel,
		RetriableErrors: []error{juju.TransactionError, juju.ConnectionRefusedError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 2 * time.Minute,
			Delay:       time.Second,
			Clock:       clock.WallClock,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create model %q, got error: %s", plan.Name, err))
		return
	}
	r.trace(fmt.Sprintf("model created : %q", modelName))

	if !plan.Cloud.IsNull() {
		// Set the cloud value if required
		newCloud := []nestedCloud{{
			Name:   types.StringValue(response.Cloud),
			Region: types.StringValue(response.CloudRegion),
		}}
		cloudType := req.Plan.Schema.GetBlocks()["cloud"].(schema.ListNestedBlock).NestedObject.Type()
		newPlanCloud, errDiag := types.ListValueFrom(ctx, cloudType, newCloud)
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		plan.Cloud = newPlanCloud
	}

	var annotations map[string]string
	resp.Diagnostics.Append(plan.Annotations.ElementsAs(ctx, &annotations, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(annotations) > 0 {
		err = r.client.Annotations.SetAnnotations(&juju.SetAnnotationsInput{
			ModelName:   modelName,
			Annotations: annotations,
			EntityTag:   names.NewModelTag(response.UUID),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set annotations for model %q, got error: %s", plan.Name, err))
			return
		}
	}

	plan.Credential = types.StringValue(response.CloudCredentialName)
	plan.Type = types.StringValue(response.Type)
	plan.UUID = types.StringValue(response.UUID)
	plan.ID = types.StringValue(response.UUID)

	r.trace(fmt.Sprintf("model resource created: %q", modelName))

	// Write the state plan into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "model", "read")
		return
	}

	var state modelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Find the model name. If the Id is a UUID, this is
	// not an Import followed by a Read. If the Id string
	// is not a UUID, find the model name in the Id, rather
	// than Name as we're doing a Read after Import. Either
	// way, we need the model name.
	var modelName string
	var imported bool
	if utils.IsValidUUIDString(state.ID.ValueString()) {
		modelName = state.Name.ValueString()
	} else {
		imported = true
		modelName = state.ID.ValueString()
	}

	response, err := r.client.Models.ReadModel(modelName)
	if err != nil {
		resp.Diagnostics.Append(handleModelNotFoundError(ctx, err, &resp.State)...)
		return
	}
	r.trace(fmt.Sprintf("found model: %v", modelName))

	// Acquire cloud, credential, and config
	tag, err := names.ParseCloudCredentialTag(response.ModelInfo.CloudCredentialTag)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse cloud credential tag for model, got error: %s", err))
		return
	}
	credential := tag.Name()

	// Set the read values into the new state model
	// Cloud
	if (imported && response.ModelInfo.CloudTag != "" && response.ModelInfo.CloudRegion != "") ||
		!state.Cloud.IsNull() {
		cloudList := []nestedCloud{{
			Name:   types.StringValue(strings.TrimPrefix(response.ModelInfo.CloudTag, juju.PrefixCloud)),
			Region: types.StringValue(response.ModelInfo.CloudRegion),
		}}
		cloudType := req.State.Schema.GetBlocks()["cloud"].(schema.ListNestedBlock).NestedObject.Type()
		newStateCloud, errDiag := types.ListValueFrom(ctx, cloudType, cloudList)
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Cloud = newStateCloud
	}

	// Constraints
	if (imported && response.ModelConstraints.String() != "") || !state.Constraints.IsNull() {
		state.Constraints = types.StringValue(response.ModelConstraints.String())
	}

	// Config
	if len(response.ModelConfig) > 0 {
		config, diags := newConfigFromModelConfigAPI(ctx, response.ModelConfig, state.Config)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Config, diags = types.MapValueFrom(ctx, types.StringType, config)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	annotations, err := r.client.Annotations.GetAnnotations(&juju.GetAnnotationsInput{
		EntityTag: names.NewModelTag(response.ModelInfo.UUID),
		ModelName: modelName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get model's annotations, got error: %s", err))
		return
	}
	if len(annotations.Annotations) > 0 {
		annotationsType := req.State.Schema.GetAttributes()["annotations"].(schema.MapAttribute).ElementType

		annotationsMapValue, errDiag := types.MapValueFrom(ctx, annotationsType, annotations.Annotations)
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		state.Annotations = annotationsMapValue
	}
	// Name, Type, Credential, and Id.
	state.Name = types.StringValue(modelName)
	state.Type = types.StringValue(response.ModelInfo.Type)
	state.Credential = types.StringValue(credential)
	state.UUID = types.StringValue(response.ModelInfo.UUID)
	state.ID = types.StringValue(response.ModelInfo.UUID)
	r.trace(fmt.Sprintf("Read model resource for: %v", modelName))
	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "model", "update")
		return
	}
	var plan, state modelResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var err error
	modelUpdate := false

	// Check config update
	var configMap map[string]string
	var unsetConfigKeys []string

	if !plan.Config.Equal(state.Config) {
		modelUpdate = true
		var diags diag.Diagnostics
		configMap, unsetConfigKeys, diags = computeConfigDiff(ctx, state.Config, plan.Config)
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}
	}

	// Check the constraints
	newConstraints, err := constraints.Parse(state.Constraints.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse constraints for model, got error: %s", err))
		return
	}
	if !plan.Constraints.Equal(state.Constraints) {
		modelUpdate = true
		newConstraints, err = constraints.Parse(plan.Constraints.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse constraints for model, got error: %s", err))
			return
		}
	}

	// Check the credential
	credentialUpdate := ""
	if !plan.Credential.Equal(state.Credential) {
		modelUpdate = true
		credentialUpdate = plan.Credential.ValueString()
	}

	// Check annotations
	if !state.Annotations.Equal(plan.Annotations) {
		resp.Diagnostics.Append(updateAnnotations(ctx, &r.client.Annotations, state.Annotations, plan.Annotations, plan.Name.ValueString(), names.NewModelTag(plan.UUID.ValueString()))...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if modelUpdate {
		var clouds []nestedCloud
		resp.Diagnostics.Append(plan.Cloud.ElementsAs(ctx, &clouds, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		var cloudNameInput string

		if len(clouds) > 0 {
			cloudNameInput = clouds[0].Name.ValueString()
		}

		err = r.client.Models.UpdateModel(juju.UpdateModelInput{
			UUID:        plan.UUID.ValueString(),
			CloudName:   cloudNameInput,
			Config:      configMap,
			Unset:       unsetConfigKeys,
			Constraints: &newConstraints,
			Credential:  credentialUpdate,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update model, got error: %s", err))
			return
		}

		r.trace(fmt.Sprintf("Updated model resource: %q", plan.Name.ValueString()))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "model", "delete")
		return
	}

	var state modelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID := state.UUID.ValueString()
	if modelUUID == "" {
		modelUUID = state.ID.ValueString()
	}
	arg := juju.DestroyModelInput{
		UUID: modelUUID,
	}
	err := r.client.Models.DestroyModel(arg)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete model, got error: %s", err))
		return
	}
	modelName := state.Name.ValueString()
	err = wait.WaitForError(wait.WaitForErrorCfg[string, *juju.ReadModelStatusResponse]{
		Context:        ctx,
		GetData:        r.client.Models.ReadModelStatus,
		Input:          modelName,
		ExpectedErr:    juju.ModelNotFoundError,
		RetryAllErrors: true,
		RetryConf: &wait.RetryConf{
			MaxDuration: maxModelDestroyWait,
		},
	})
	if err != nil {
		errSummary := "Client Error"
		errDetail := fmt.Sprintf("Unable to complete model %q deletion: %v\n", modelName, err)
		if r.config.SkipFailedDeletion {
			resp.Diagnostics.AddWarning(
				errSummary,
				errDetail+"There might be dangling resources requiring manual intervion.\n",
			)
		} else {
			resp.Diagnostics.AddError(
				errSummary,
				errDetail,
			)
		}

		return
	}
	r.trace(fmt.Sprintf("model deleted : %q", modelName))
}

func handleModelNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	// This should not happen anymore, because Delete waits for the model to be destroyed.
	if errors.As(err, &juju.ModelNotFoundError) {
		// Model manually removed
		st.RemoveResource(ctx)
		return diag.Diagnostics{}
	}

	var diags diag.Diagnostics
	diags.AddError("Client Error", err.Error())
	return diags
}

func (r *modelResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(r.subCtx, LogResourceModel, msg, additionalFields...)
}
