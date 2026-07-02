// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/api/client/action"
	"github.com/juju/juju/rpc/params"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

var _ resource.Resource = &actionResource{}
var _ resource.ResourceWithConfigure = &actionResource{}

// NewActionResource returns a new action resource.
func NewActionResource() resource.Resource {
	return &actionResource{}
}

type actionResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for actions.
	subCtx context.Context
}

// actionResourceModel is the model for the juju_action resource.
type actionResourceModel struct {
	// ModelUUID is the UUID of the model where the action will be run.
	ModelUUID types.String `tfsdk:"model_uuid"`
	// ApplicationName is the name of the application to run the action on.
	ApplicationName types.String `tfsdk:"application_name"`
	// ActionName is the name of the action to run.
	ActionName types.String `tfsdk:"action_name"`
	// Unit is the unit name (e.g. "ubuntu/0") to run the action on. If not
	// provided, the leader unit of the application is targeted.
	Unit types.String `tfsdk:"unit"`
	// Args are the arguments to pass to the action.
	Args types.Map `tfsdk:"args"`
	// ActionID is the ID of the enqueued action. It is computed after the
	// action has been enqueued.
	ActionID types.String `tfsdk:"action_id"`
	// Output is the output of the action. It is a map of strings, as
	// aligned with the way action results are set by charms.
	Output types.Map `tfsdk:"output"`
	// ID required by the testing framework.
	ID types.String `tfsdk:"id"`
}

// Metadata returns the full resource name as used in terraform plans.
func (r *actionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_action"
}

// Schema returns the schema for the action resource.
func (r *actionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju action. The action is run and its result awaited during the resource's creation. " +
			"The action's output is set as a computed field that can be used by other resources after the resource has been created.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the action will be run. Changing this value will cause the resource to be destroyed and recreated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"application_name": schema.StringAttribute{
				Description: "The name of the application to run the action on. Changing this value will cause the resource to be destroyed and recreated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"action_name": schema.StringAttribute{
				Description: "The name of the action to run. Changing this value will cause the resource to be destroyed and recreated.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"unit": schema.StringAttribute{
				Description: "The unit name (e.g. \"ubuntu/0\") to run the action on. If not provided, the leader unit of the application is targeted. Changing this value will cause the resource to be destroyed and recreated.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"args": schema.MapAttribute{
				Description: "The arguments to pass to the action. Changing this value will cause the resource to be destroyed and recreated.",
				Optional:    true,
				ElementType: types.StringType,
			},
			"action_id": schema.StringAttribute{
				Description: "The ID of the enqueued action.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"output": schema.MapAttribute{
				ElementType: types.StringType,
				Description: "The output of the action. It is a map of strings, as aligned with the way action results are set by charms.",
				Computed:    true,
			},
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// resource.
func (r *actionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceAction)
}

// Create enqueues the action and waits for its completion.
func (r *actionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "action", "create")
		return
	}

	var plan actionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID := plan.ModelUUID.ValueString()
	appName := plan.ApplicationName.ValueString()
	actionName := plan.ActionName.ValueString()

	// Resolve the receiver unit. If no unit is provided, target the leader.
	receiver := plan.Unit.ValueString()
	if receiver == "" {
		var err error
		receiver, err = r.client.Actions.ResolveLeaderUnit(ctx, juju.ResolveLeaderUnitArgs{
			ModelUUID:       modelUUID,
			ApplicationName: appName,
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to resolve leader unit for application %q: %s", appName, err))
			return
		}
		plan.Unit = types.StringValue(receiver)
	}

	// Build the action parameters from the args map.
	actionParams := make(map[string]interface{})
	if !plan.Args.IsNull() && !plan.Args.IsUnknown() {
		argsMap := make(map[string]string, len(plan.Args.Elements()))
		resp.Diagnostics.Append(plan.Args.ElementsAs(ctx, &argsMap, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for k, v := range argsMap {
			actionParams[k] = v
		}
	}

	actionID, err := r.client.Actions.EnqueueAction(ctx, juju.EnqueueActionArgs{
		ModelUUID:  modelUUID,
		Receiver:   receiver,
		Name:       actionName,
		Parameters: actionParams,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to enqueue action %q: %s", actionName, err))
		return
	}
	plan.ActionID = types.StringValue(actionID)
	plan.ID = types.StringValue(newActionResourceID(modelUUID, appName, actionName, actionID))

	// Save the state with the action_id before waiting for the result.
	// This way, if the wait fails and the resource is tainted, the
	// action_id is already in state and a subsequent apply's Read can
	// wait for the existing action's result instead of re-enqueuing.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Wait for the action to complete and populate the output.
	actionResult, err := waitActionResult(ctx, r, modelUUID, actionID, actionName)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for action %q to complete: %s", actionName, err))
		return
	}

	// Set the output map.
	plan.Output = actionResultToOutputMap(ctx, actionResult, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read reads the action result and updates the state.
func (r *actionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "action", "read")
		return
	}

	var state actionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If the output is not yet populated, the action may still be running,
	// this can happen when the wait fails after the action has been enqueued.
	// Wait for the action to complete and populate the output.
	if state.Output.IsNull() || state.Output.IsUnknown() {
		actionResult, err := waitActionResult(ctx, r, state.ModelUUID.ValueString(), state.ActionID.ValueString(), state.ActionName.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for action %q to complete: %s", state.ActionName.ValueString(), err))
			return
		}
		state.Output = actionResultToOutputMap(ctx, actionResult, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is a no-op for the action resource. All schema attributes use
// RequiresReplace, so any change causes the resource to be destroyed and
// recreated rather than updated.
func (r *actionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
}

// Delete is a no-op for the action resource. Actions cannot be deleted
// from Juju, so we just remove the resource from the Terraform state.
func (r *actionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

// assertActionCompleted asserts that the action has completed successfully.
// If the action is still running or pending, it returns a retry error.
// If the action failed, was cancelled, or errored, it returns a fatal error.
func assertActionCompleted(resultFromAPI action.ActionResult) error {
	if resultFromAPI.Error != nil {
		return resultFromAPI.Error
	}
	switch resultFromAPI.Status {
	case params.ActionRunning, params.ActionPending:
		return juju.NewRetryReadError("action is still running or pending, waiting for completion")
	case params.ActionCompleted:
		return nil
	default:
		return errors.New("action did not complete successfully, status: " + resultFromAPI.Status)
	}
}

// waitActionResult waits for the action identified by actionID to complete
// and returns its result.
func waitActionResult(ctx context.Context, r *actionResource, modelUUID, actionID, actionName string) (action.ActionResult, error) {
	return wait.WaitFor(wait.WaitForCfg[juju.ActionResultArgs, action.ActionResult]{
		Context: ctx,
		Input: juju.ActionResultArgs{
			ModelUUID: modelUUID,
			ActionID:  actionID,
		},
		GetData:        r.client.Actions.ActionResult,
		DataAssertions: []wait.Assert[action.ActionResult]{assertActionCompleted},
		NonFatalErrors: []error{juju.RetryReadError},
		Logf: func(msg string, additionalFields ...map[string]interface{}) {
			tflog.SubsystemDebug(r.subCtx, LogResourceAction, msg, additionalFields...)
		},
	})
}

// actionResultToOutputMap converts an action result's output into a map of
// strings suitable for storing in Terraform state.
func actionResultToOutputMap(ctx context.Context, actionResult action.ActionResult, diags *diag.Diagnostics) types.Map {
	outputMap := make(map[string]types.String, len(actionResult.Output))
	for k, v := range actionResult.Output {
		outputMap[k] = types.StringValue(fmt.Sprint(v))
	}
	result, d := types.MapValueFrom(ctx, types.StringType, outputMap)
	diags.Append(d...)
	return result
}

// newActionResourceID builds the resource ID from its components.
// The ID is composed of the model UUID, application name, action name and
// the enqueued action ID, separated by ":".
func newActionResourceID(modelUUID, appName, actionName, actionID string) string {
	return fmt.Sprintf("%s:%s:%s:%s", modelUUID, appName, actionName, actionID)
}
