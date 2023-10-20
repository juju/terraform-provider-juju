// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
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
	Name           types.String `tfsdk:"name"`
	ModelName      types.String `tfsdk:"model"`
	Constraints    types.String `tfsdk:"constraints"`
	Disks          types.String `tfsdk:"disks"`
	Base           types.String `tfsdk:"base"`
	Series         types.String `tfsdk:"series"`
	MachineID      types.String `tfsdk:"machine_id"`
	SSHAddress     types.String `tfsdk:"ssh_address"`
	PublicKeyFile  types.String `tfsdk:"public_key_file"`
	PrivateKeyFile types.String `tfsdk:"private_key_file"`
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
			NameKey: schema.StringAttribute{
				Description: "A name for the machine resource in Terraform.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			ModelKey: schema.StringAttribute{
				Description: "The Juju model in which to add a new machine.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			ConstraintsKey: schema.StringAttribute{
				Description: "Machine constraints that overwrite those available from 'juju get-model-constraints' and provider's defaults.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplaceIfConfigured(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot(SSHAddressKey),
					}...),
				},
			},
			DisksKey: schema.StringAttribute{
				Description: "Storage constraints for disks to attach to the machine(s).",
				Optional:    true,
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
				Description: "The operating system to install on the new machine(s). E.g. ubuntu@22.04.",
				Optional:    true,
				Computed:    true,
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
				Description: "The operating system series to install on the new machine(s).",
				Optional:    true,
				Computed:    true,
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
				DeprecationMessage: "Configure base instead. This attribute will be removed in the next major version of the provider.",
			},
			MachineIDKey: schema.StringAttribute{
				Description: "The id of the machine Juju creates.",
				Computed:    true,
				Optional:    false,
				Required:    false,
			},
			SSHAddressKey: schema.StringAttribute{
				Description: "The user@host directive for manual provisioning an existing machine via ssh. " +
					"Requires public_key_file & private_key_file arguments.",
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

	var data machineResourceModel
	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	response, err := r.client.Machines.CreateMachine(&juju.CreateMachineInput{
		Constraints:    data.Constraints.ValueString(),
		ModelName:      data.ModelName.ValueString(),
		Disks:          data.Disks.ValueString(),
		Base:           data.Base.ValueString(),
		Series:         data.Series.ValueString(),
		SSHAddress:     data.SSHAddress.ValueString(),
		PublicKeyFile:  data.PublicKeyFile.ValueString(),
		PrivateKeyFile: data.PrivateKeyFile.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create machine, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("create machine resource %q", response.ID))

	machineName := data.Name.ValueString()
	if machineName == "" {
		machineName = fmt.Sprintf("machine-%s", response.ID)
	}

	id := newMachineID(data.ModelName.ValueString(), response.ID, machineName)
	data.ID = types.StringValue(id)
	data.MachineID = types.StringValue(response.ID)
	data.Base = types.StringValue(response.Base)
	data.Series = types.StringValue(response.Series)
	data.Name = types.StringValue(machineName)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
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

	data.Name = types.StringValue(machineName)
	data.ModelName = types.StringValue(modelName)
	data.MachineID = types.StringValue(machineID)
	data.Series = types.StringValue(response.Series)
	data.Base = types.StringValue(response.Base)
	if response.Constraints != "" {
		data.Constraints = types.StringValue(response.Constraints)
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

	// Only the name can be updated it is terraform data and
	// not saved in juju.
	if plan.Name.Equal(state.Name) {
		return
	}
	state.Name = plan.Name
	id := newMachineID(plan.ModelName.ValueString(), plan.MachineID.ValueString(), plan.Name.ValueString())
	state.ID = types.StringValue(id)

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
