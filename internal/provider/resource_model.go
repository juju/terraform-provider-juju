// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	frameworkdiag "github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/names/v4"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ frameworkresource.Resource = &modelResource{}
var _ frameworkresource.ResourceWithConfigure = &modelResource{}
var _ frameworkresource.ResourceWithImportState = &modelResource{}

func NewModelResource() frameworkresource.Resource {
	return &modelResource{}
}

type modelResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type modelResourceModel struct {
	Name        types.String `tfsdk:"name"`
	Cloud       types.List   `tfsdk:"cloud"`
	Config      types.Map    `tfsdk:"config"`
	Constraints types.String `tfsdk:"constraints"`
	Credential  types.String `tfsdk:"credential"`
	Type        types.String `tfsdk:"type"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// nestedCloud represents an element in a Cloud list of a model resource
type nestedCloud struct {
	Name   types.String `tfsdk:"name"`
	Region types.String `tfsdk:"region"`
}

func (r *modelResource) Schema(_ context.Context, _ frameworkresource.SchemaRequest, resp *frameworkresource.SchemaResponse) {
	resp.Schema = frameworkschema.Schema{
		Description: "A resource that represent a Juju Model.",
		Attributes: map[string]frameworkschema.Attribute{
			"name": frameworkschema.StringAttribute{
				Description: "The name to be assigned to the model",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cloud": frameworkschema.ListNestedAttribute{
				Description: "JuJu Cloud where the model will operate",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: frameworkschema.NestedAttributeObject{
					Attributes: map[string]frameworkschema.Attribute{
						"name": frameworkschema.StringAttribute{
							Description: "The name of the cloud",
							Required:    true,
						},
						"region": frameworkschema.StringAttribute{
							Description: "The region of the cloud",
							Optional:    true,
							Computed:    true,
						},
					},
				},
			},
			"config": frameworkschema.MapAttribute{
				Description: "Override default model configuration",
				Optional:    true,
			},
			"constraints": frameworkschema.StringAttribute{
				Description: "Constraints imposed to this model",
				Optional:    true,
			},
			"credential": frameworkschema.StringAttribute{
				Description: "Credential used to add the model",
				Optional:    true,
				Computed:    true,
			},
			"type": frameworkschema.StringAttribute{
				Description: "Type of the model. Set by the Juju's API server",
				Computed:    true,
			},
			"id": frameworkschema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *modelResource) Configure(ctx context.Context, req frameworkresource.ConfigureRequest, resp *frameworkresource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceModel)
}

func (r *modelResource) Metadata(_ context.Context, req frameworkresource.MetadataRequest, resp *frameworkresource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_model"
}

func (r *modelResource) ImportState(ctx context.Context, req frameworkresource.ImportStateRequest, resp *frameworkresource.ImportStateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "model", "import")
		return
	}

	// Import command takes the model name as an argument
	// `terraform import juju_model.RESOURCE_NAME MODEL_NAME`
	modelName := req.ID

	// We set the ID to the modelUUID here
	modelInfo, err := r.client.Models.GetModelByName(modelName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to find model, got error: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("name"), modelInfo.Name)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), modelInfo.UUID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.trace(fmt.Sprintf("Imported model resource: %v", modelInfo.Name))
}

func (r *modelResource) Create(ctx context.Context, req frameworkresource.CreateRequest, resp *frameworkresource.CreateResponse) {
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
	resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &clouds, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config map[string]interface{}
	resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &config, false)...)
	if resp.Diagnostics.HasError() {
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

	cloudNameInput := clouds[0].Name.ValueString()
	cloudRegionInput := clouds[0].Region.ValueString()

	response, err := r.client.Models.CreateModel(juju.CreateModelInput{
		Name:        modelName,
		CloudName:   cloudNameInput,
		CloudRegion: cloudRegionInput,
		Config:      config,
		Constraints: parsedConstraints,
		Credential:  credential,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create model, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("model created : %q", modelName))

	cloudChanged := false
	var cloudNewName, cloudNewRegion string
	if cloudNameInput == "" {
		// no cloud value was defined, use the response
		cloudNewName = response.ModelInfo.Cloud
		cloudChanged = true
	}
	if cloudRegionInput == "" {
		cloudNewRegion = response.ModelInfo.CloudRegion
		cloudChanged = true
	}

	// TODO: Should config track all key=value or just those explicitly set?

	// Set the cloud value if required
	if cloudChanged {
		newCloud := []nestedCloud{{
			Name:   types.StringValue(cloudNewName),
			Region: types.StringValue(cloudNewRegion),
		}}
		cloudType := req.Plan.Schema.GetAttributes()["cloud"].(frameworkschema.ListNestedAttribute).NestedObject.Type()
		newPlanCloud, errDiag := types.ListValueFrom(ctx, cloudType, newCloud)
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		plan.Cloud = newPlanCloud
	}
	plan.ID = types.StringValue(response.ModelInfo.UUID)

	r.trace(fmt.Sprintf("model resource created: %q", modelName))

	// Write the state plan into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modelResource) Read(ctx context.Context, req frameworkresource.ReadRequest, resp *frameworkresource.ReadResponse) {
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

	uuid := state.ID.ValueString()
	response, err := r.client.Models.ReadModel(uuid)
	if err != nil {
		resp.Diagnostics.Append(handleModelNotFoundError(ctx, err, &resp.State)...)
		return
	}
	r.trace(fmt.Sprintf("found model: %v", uuid))

	// Acquire cloud, credential, and config
	cloudList := []nestedCloud{{
		Name:   types.StringValue(strings.TrimPrefix(response.ModelInfo.CloudTag, juju.PrefixCloud)),
		Region: types.StringValue(response.ModelInfo.CloudRegion),
	}}
	cloudType := req.State.Schema.GetAttributes()["cloud"].(frameworkschema.ListNestedAttribute).NestedObject.Type()
	newStateCloud, errDiag := types.ListValueFrom(ctx, cloudType, cloudList)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}

	tag, err := names.ParseCloudCredentialTag(response.ModelInfo.CloudCredentialTag)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse cloud credential tag for model, got error: %s", err))
		return
	}
	credential := tag.Name()

	// Only read model config that is tracked in Terraform
	var stateConfig map[string]interface{}
	resp.Diagnostics.Append(state.Config.ElementsAs(ctx, &stateConfig, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	for k := range stateConfig {
		if value, exists := response.ModelConfig[k]; exists {
			var serialised string
			switch value.(type) {
			// TODO: review for other possible types
			case bool:
				b, err := json.Marshal(value)
				if err != nil {
					resp.Diagnostics.AddError("Provider Error", fmt.Sprintf("Unable to cast config value, got error: %s", err))
					return
				}
				serialised = string(b)
			default:
				serialised = value.(string)
			}

			stateConfig[k] = serialised
		}
	}

	configType := req.State.Schema.GetAttributes()["config"].(frameworkschema.MapAttribute).ElementType
	newStateConfig, errDiag := types.MapValueFrom(ctx, configType, stateConfig)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Set the read values into the new state model
	state.Cloud = newStateCloud
	state.Name = types.StringValue(response.ModelInfo.Name)
	state.Constraints = types.StringValue(response.ModelConstraints.String())
	state.Credential = types.StringValue(credential)
	state.Config = newStateConfig

	r.trace(fmt.Sprintf("Read model resource: %v", state.ID.ValueString()))
	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *modelResource) Update(ctx context.Context, req frameworkresource.UpdateRequest, resp *frameworkresource.UpdateResponse) {
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
	noChange := true

	// items that could be changed
	var newConfigMap map[string]interface{}
	var newConstraints constraints.Value
	var unsetConfigKeys []string
	var newCredential string

	// Check config update
	if !plan.Config.Equal(state.Config) {
		noChange = false

		var oldConfigMap map[string]interface{}
		resp.Diagnostics.Append(state.Config.ElementsAs(ctx, &oldConfigMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		var newConfigMap map[string]interface{}
		resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &newConfigMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for k := range oldConfigMap {
			if _, ok := newConfigMap[k]; !ok {
				unsetConfigKeys = append(unsetConfigKeys, k)
			}
		}
	}

	// Check the constraints
	if !plan.Constraints.Equal(state.Constraints) {
		noChange = false
		newConstraints, err = constraints.Parse(plan.Constraints.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse constraints for model, got error: %s", err))
			return
		}
	}

	// Check the credential
	if !plan.Credential.Equal(state.Credential) {
		noChange = false
		newCredential = plan.Credential.ValueString()
	}

	if noChange {
		return
	}

	var clouds []interface{}
	resp.Diagnostics.Append(plan.Config.ElementsAs(ctx, &clouds, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err = r.client.Models.UpdateModel(juju.UpdateModelInput{
		UUID:        plan.ID.ValueString(),
		CloudList:   clouds,
		Config:      newConfigMap,
		Unset:       unsetConfigKeys,
		Constraints: &newConstraints,
		Credential:  newCredential,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update model, got error: %s", err))
		return
	}

	r.trace(fmt.Sprintf("Updated model resource: %q", plan.ID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *modelResource) Delete(ctx context.Context, req frameworkresource.DeleteRequest, resp *frameworkresource.DeleteResponse) {
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

	err := r.client.Models.DestroyModel(juju.DestroyModelInput{
		UUID: state.ID.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete model, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("model deleted : %q", state.ID.ValueString()))
}

func handleModelNotFoundError(ctx context.Context, err error, st *tfsdk.State) frameworkdiag.Diagnostics {
	if errors.As(err, &juju.ModelNotFoundError) {
		// Model manually removed
		st.RemoveResource(ctx)
		return frameworkdiag.Diagnostics{}
	}

	var diags frameworkdiag.Diagnostics
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
