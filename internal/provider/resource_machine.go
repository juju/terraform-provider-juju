// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/clock"
	"github.com/juju/juju/core/status"
	"github.com/juju/names/v5"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &machineResource{}
var _ resource.ResourceWithConfigure = &machineResource{}
var _ resource.ResourceWithImportState = &machineResource{}

func NewMachineResource() resource.Resource {
	return &machineResource{}
}

type machineResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type machineResourceModel struct {
	Annotations     types.Map              `tfsdk:"annotations"`
	Name            types.String           `tfsdk:"name"`
	ModelName       types.String           `tfsdk:"model"`
	Constraints     CustomConstraintsValue `tfsdk:"constraints"`
	Disks           types.String           `tfsdk:"disks"`
	Base            types.String           `tfsdk:"base"`
	Series          types.String           `tfsdk:"series"`
	Placement       types.String           `tfsdk:"placement"`
	MachineID       types.String           `tfsdk:"machine_id"`
	SSHAddress      types.String           `tfsdk:"ssh_address"`
	PublicKeyFile   types.String           `tfsdk:"public_key_file"`
	PrivateKeyFile  types.String           `tfsdk:"private_key_file"`
	Hostname        types.String           `tfsdk:"hostname"`
	WaitForHostname types.Bool             `tfsdk:"wait_for_hostname"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (r *machineResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_machine"
}

// Configure enables provider-level data or clients to be set in the
// provider-defined DataSource type. It is separately executed for each
// ReadDataSource RPC.
func (r *machineResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	// Create the local logging subsystem here, using the TF context when creating it.
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceMachine)
}

const (
	NameKey           = "name"
	ModelKey          = "model"
	ConstraintsKey    = "constraints"
	DisksKey          = "disks"
	SeriesKey         = "series"
	PlacementKey      = "placement"
	BaseKey           = "base"
	MachineIDKey      = "machine_id"
	SSHAddressKey     = "ssh_address"
	PrivateKeyFileKey = "private_key_file"
	PublicKeyFileKey  = "public_key_file"
)

func (r *machineResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju machine deployment. Refer to the juju add-machine CLI command for more information and limitations.",
		Attributes: map[string]schema.Attribute{
			"annotations": schema.MapAttribute{
				Description: "Annotations are key/value pairs that can be used to store additional information about the machine. May not contain dots (.) in keys.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			NameKey: schema.StringAttribute{
				Description: "A name for the machine resource in Terraform.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			ModelKey: schema.StringAttribute{
				Description: "The Juju model in which to add a new machine. Changing this value will cause the machine" +
					" to be destroyed and recreated by terraform.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			ConstraintsKey: schema.StringAttribute{
				CustomType: CustomConstraintsType{},
				Description: "Machine constraints that overwrite those available from 'juju get-model-constraints' " +
					"and provider's defaults. Changing this value will cause the application to be destroyed and" +
					" recreated by terraform.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIf(constraintsRequiresReplacefunc, "", ""),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
					}...),
				},
			},
			DisksKey: schema.StringAttribute{
				Description: "Storage constraints for disks to attach to the machine(s). Changing this" +
					" value will cause the machine to be destroyed and recreated by terraform.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
					}...),
				},
			},
			BaseKey: schema.StringAttribute{
				Description: "The operating system to install on the new machine(s). E.g. ubuntu@22.04. Changing this" +
					" value will cause the machine to be destroyed and recreated by terraform.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
						path.MatchRoot(SeriesKey),
					}...),
					stringIsBaseValidator{},
				},
			},
			SeriesKey: schema.StringAttribute{
				Description: "The operating system series to install on the new machine(s). Changing this value" +
					" will cause the machine to be destroyed and recreated by terraform.",
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
						path.MatchRoot(BaseKey),
					}...),
				},
				DeprecationMessage: "Configure base instead. This attribute will be removed in the next" +
					" major version of the provider.",
			},
			PlacementKey: schema.StringAttribute{
				Description: "Additional information about how to allocate the machine in the cloud. Changing" +
					" this value will cause the application to be destroyed and recreated by terraform.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
					}...),
				},
			},
			MachineIDKey: schema.StringAttribute{
				Description: "The id of the machine Juju creates.",
				Computed:    true,
				Optional:    false,
				Required:    false,
			},
			SSHAddressKey: schema.StringAttribute{
				Description: "The user@host directive for manual provisioning an existing machine via ssh. " +
					"Requires public_key_file & private_key_file arguments. Changing this value will cause the" +
					" machine to be destroyed and recreated by terraform.",
				Optional: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SeriesKey),
						path.MatchRoot(BaseKey),
						path.MatchRoot(ConstraintsKey),
					}...),
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRoot(PublicKeyFileKey),
						path.MatchRoot(PrivateKeyFileKey),
					}...),
				},
			},
			PublicKeyFileKey: schema.StringAttribute{
				Description: "The file path to read the public key from.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRoot(SSHAddressKey),
						path.MatchRoot(PrivateKeyFileKey),
					}...),
				},
			},
			PrivateKeyFileKey: schema.StringAttribute{
				Description: "The file path to read the private key from.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.Expressions{
						path.MatchRoot(PublicKeyFileKey),
						path.MatchRoot(SSHAddressKey),
					}...),
				},
			},
			"hostname": schema.StringAttribute{
				Description: "The machine's hostname. This is set only if 'wait_for_hostname' is true.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"wait_for_hostname": schema.BoolAttribute{
				Description: "If true, waits for the machine's hostname to be set during creation. " +
					"A side effect is that this also waits for the machine to reach 'active' state in Juju.",
				Optional: true,
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

// Create is called when the provider must create a new resource. Config
// and planned state values should be read from the
// CreateRequest and new state values set on the CreateResponse.
func (r *machineResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
		return
	}

	var plan machineResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.Machines.CreateMachine(ctx, &juju.CreateMachineInput{
		Constraints:    plan.Constraints.ValueString(),
		ModelName:      plan.ModelName.ValueString(),
		Disks:          plan.Disks.ValueString(),
		Base:           plan.Base.ValueString(),
		Series:         plan.Series.ValueString(),
		SSHAddress:     plan.SSHAddress.ValueString(),
		Placement:      plan.Placement.ValueString(),
		PublicKeyFile:  plan.PublicKeyFile.ValueString(),
		PrivateKeyFile: plan.PrivateKeyFile.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create machine, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("create machine resource %q", response.ID))

	var annotations map[string]string
	resp.Diagnostics.Append(plan.Annotations.ElementsAs(ctx, &annotations, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(annotations) > 0 {
		err = r.client.Annotations.SetAnnotations(&juju.SetAnnotationsInput{
			ModelName:   plan.ModelName.ValueString(),
			Annotations: annotations,
			EntityTag:   names.NewMachineTag(response.ID),
		})
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set annotations for machine %q, got error: %s", response.ID, err))
			return
		}
	}

	machineName := plan.Name.ValueString()
	if machineName == "" {
		machineName = fmt.Sprintf("machine-%s", response.ID)
	}

	id := newMachineID(plan.ModelName.ValueString(), response.ID, machineName)
	plan.ID = types.StringValue(id)
	plan.MachineID = types.StringValue(response.ID)
	plan.Name = types.StringValue(machineName)
	plan.Hostname = types.StringValue("")
	asserts := []wait.Assert[juju.ReadMachineResponse]{assertMachineRunning}
	if plan.WaitForHostname.ValueBool() {
		asserts = append(asserts, assertHostnamePopulated)
	}
	readResponse, err := wait.WaitFor(wait.WaitForCfg[juju.ReadMachineInput, juju.ReadMachineResponse]{
		Context: ctx,
		GetData: r.client.Machines.ReadMachine,
		Input: juju.ReadMachineInput{
			ModelName: plan.ModelName.ValueString(),
			ID:        response.ID,
		},
		DataAssertions: asserts,
		NonFatalErrors: []error{juju.RetryReadError, juju.ConnectionRefusedError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 30 * time.Minute,
			Delay:       juju.ReadModelDefaultInterval,
			Clock:       clock.WallClock,
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for hostname to be set on machine %q, got error: %s", response.ID, err))
		return
	}
	plan.Base = types.StringValue(readResponse.Base)
	plan.Series = types.StringValue(readResponse.Series)
	plan.Hostname = types.StringValue(readResponse.Hostname)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func IsMachineNotFound(err error) bool {
	return strings.Contains(err.Error(), "no status returned for machine")
}

func handleMachineNotFoundError(ctx context.Context, err error, st *tfsdk.State) diag.Diagnostics {
	if IsMachineNotFound(err) {
		// Machine manually removed
		st.RemoveResource(ctx)
		return diag.Diagnostics{}
	}
	var diags diag.Diagnostics
	diags.AddError("Not Found", err.Error())
	return diags
}

// Read is called when the provider must read resource values in order
// to update state. Planned state values should be read from the
// ReadRequest and new state values set on the ReadResponse.
// Take the juju api input from the ID, it may not exist in the plan.
// Only set optional values if they exist.
func (r *machineResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
		return
	}

	var data machineResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, machineID, machineName := modelMachineIDAndName(data.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.Machines.ReadMachine(juju.ReadMachineInput{
		ModelName: modelName,
		ID:        machineID,
	})
	if err != nil {
		resp.Diagnostics.Append(handleMachineNotFoundError(ctx, err, &resp.State)...)
		return
	}
	r.trace(fmt.Sprintf("read machine resource %q", machineID))

	annotations, err := r.client.Annotations.GetAnnotations(&juju.GetAnnotationsInput{
		EntityTag: names.NewMachineTag(response.ID),
		ModelName: modelName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get machine's annotations, got error: %s", err))
		return
	}
	if len(annotations.Annotations) > 0 {
		annotationsType := req.State.Schema.GetAttributes()["annotations"].(schema.MapAttribute).ElementType

		annotationsMapValue, errDiag := types.MapValueFrom(ctx, annotationsType, annotations.Annotations)
		resp.Diagnostics.Append(errDiag...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Annotations = annotationsMapValue
	}

	data.Name = types.StringValue(machineName)
	data.ModelName = types.StringValue(modelName)
	data.MachineID = types.StringValue(machineID)
	data.Series = types.StringValue(response.Series)
	data.Base = types.StringValue(response.Base)
	// Here is ok to always set Hostname because:
	// 1. if you set wait_for_hostname to true, this is correctly populated.
	// 2. if you set wait_for_hostname to false, you shouldn't use the hostname.
	// 3. if you import a machine, the hostname should have been already populated.
	//    It could happen that the hostname is set to an empty string during import, but it is very unlikely because
	//    that means you've created a machine and then imported it immediately afterwards.
	data.Hostname = types.StringValue(response.Hostname)
	if response.Constraints != "" {
		data.Constraints = NewCustomConstraintsValue(response.Constraints)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is called to update the state of the resource. Config, planned
// state, and prior state values should be read from the
// UpdateRequest and new state values set on the UpdateResponse.
func (r *machineResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state machineResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO hml 28-Jul-2023
	// Delete the machine resource if it no longer exists in juju.

	// Check annotations
	if !state.Annotations.Equal(plan.Annotations) {
		resp.Diagnostics.Append(updateAnnotations(ctx, &r.client.Annotations, state.Annotations, plan.Annotations, state.ModelName.ValueString(), names.NewMachineTag(state.MachineID.ValueString()))...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// Only the name or annotations can be updated in the terraform data.
	if plan.Name.Equal(state.Name) && state.Annotations.Equal(plan.Annotations) {
		return
	}
	state.Name = plan.Name
	id := newMachineID(plan.ModelName.ValueString(), state.MachineID.ValueString(), plan.Name.ValueString())
	state.ID = types.StringValue(id)
	state.Annotations = plan.Annotations

	r.trace(fmt.Sprintf("update machine resource %q", plan.MachineID.ValueString()))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Delete is called when the provider must delete the resource. Config
// values may be read from the DeleteRequest.
//
// If execution completes without error, the framework will automatically
// call DeleteResponse.State.RemoveResource(), so it can be omitted
// from provider logic.
//
// Juju refers to deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func (r *machineResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
		return
	}

	var data machineResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelName, machineID, _ := modelMachineIDAndName(data.ID.ValueString(), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Machines.DestroyMachine(&juju.DestroyMachineInput{
		ModelName: modelName,
		ID:        machineID,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete machine, got error: %s", err))
	}
	r.trace(fmt.Sprintf("delete machine resource %q", machineID))
}

// ImportState is called when the provider must import the state of a
// resource instance. This method must return enough state so the Read
// method can properly refresh the full resource.
//
// If setting an attribute with the import identifier, it is recommended
// to use the ImportStatePassthroughID() call in this method.
func (r *machineResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *machineResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(r.subCtx, LogResourceMachine, msg, additionalFields...)
}

func newMachineID(model, machine_id, machine_name string) string {
	return fmt.Sprintf("%s:%s:%s", model, machine_id, machine_name)
}

// Machines can be imported using the format: `model_name:machine_id:machine_name`.
func modelMachineIDAndName(value string, diags *diag.Diagnostics) (string, string, string) {
	id := strings.Split(value, ":")
	//If importing with an incorrect ID we need to catch and provide a user-friendly error
	if len(id) != 3 {
		diags.AddError("Malformed ID", fmt.Sprintf("unable to parse model name, machine id, and machine name from provided ID: %q", value))
		return "", "", ""
	}
	return id[0], id[1], id[2]
}

type annotationSetter interface {
	SetAnnotations(input *juju.SetAnnotationsInput) error
}

// updateAnnotations takes the state and the plan, and performs the necessary
// steps to propagate the changes to juju.
func updateAnnotations(ctx context.Context, client annotationSetter, stateAnnotations types.Map, planAnnotations types.Map, modelName string, entityTag names.Tag) diag.Diagnostics {
	diagnostics := diag.Diagnostics{}

	var annotationsState map[string]string
	diagnostics.Append(stateAnnotations.ElementsAs(ctx, &annotationsState, false)...)
	if diagnostics.HasError() {
		return diagnostics
	}
	var annotationsPlan map[string]string
	diagnostics.Append(planAnnotations.ElementsAs(ctx, &annotationsPlan, false)...)
	if diagnostics.HasError() {
		return diagnostics
	}
	// when the plan is empty this map is nil, instead of being initialized with 0 items.
	if annotationsPlan == nil {
		annotationsPlan = make(map[string]string, 0)
	}
	// set the value of removed fields to "" in the plan to unset the value
	for k := range annotationsState {
		if _, ok := annotationsPlan[k]; !ok {
			annotationsPlan[k] = ""
		}
	}

	err := client.SetAnnotations(&juju.SetAnnotationsInput{
		ModelName:   modelName,
		Annotations: annotationsPlan,
		EntityTag:   entityTag,
	})
	if err != nil {
		diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set annotations for model %q, got error: %s", modelName, err))
		return diagnostics
	}
	return diagnostics
}

// assertHostnamePopulated asserts the hostname is populated in the machine response.
// Otherwise it returns a retry error to wait for the hostname to be set.
func assertHostnamePopulated(respFromAPI juju.ReadMachineResponse) error {
	if respFromAPI.Hostname == "" {
		return juju.NewRetryReadError("waiting for hostname to be set on machine")
	}
	return nil
}

// assertMachineRunning asserts that the machine is in the running state, otherwise it returns a retry error.
// This is important when using the placement directive in juju_application resource - to deploy an application
// or validate against the operating system specified for the application Juju must know the operating system to use.
// For actual machines that information is not available until it reaches the "running" state.
func assertMachineRunning(respFromAPI juju.ReadMachineResponse) error {
	if respFromAPI.Status != status.Running.String() {
		return juju.NewRetryReadError("waiting for machine to be in running state")
	}
	return nil
}
