// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/names/v5"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

var _ resource.Resource = &subnetResource{}
var _ resource.ResourceWithConfigure = &subnetResource{}
var _ resource.ResourceWithImportState = &subnetResource{}
var _ resource.ResourceWithIdentity = &subnetResource{}

// NewSubnetResource returns a new subnet resource.
func NewSubnetResource() resource.Resource {
	return &subnetResource{}
}

type subnetResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type subnetResourceModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	CIDR      types.String `tfsdk:"cidr"`
	SpaceName types.String `tfsdk:"space_name"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type subnetResourceIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

// Metadata implements [resource.Resource].
func (r *subnetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_subnet"
}

// Schema implements [resource.Resource].
func (r *subnetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents assignment of a subnet to a Juju space.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the subnet exists.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"cidr": schema.StringAttribute{
				Description: "The subnet CIDR. Changing this value forces replacement.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"space_name": schema.StringAttribute{
				Description: "The target space for this subnet.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidSpace, "must be a valid space name"),
				},
			},
			"id": schema.StringAttribute{
				Description: "The identifier of the subnet resource. Format: <model_uuid>:<cidr>",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// IdentitySchema implements [resource.ResourceWithIdentity].
func (r *subnetResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
			},
		},
	}
}

// Configure implements [resource.ResourceWithConfigure].
func (r *subnetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSubnet)
}

// ImportState implements [resource.ResourceWithImportState].
func (r *subnetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

// Create implements [resource.Resource].
func (r *subnetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "subnet", "create")
		return
	}

	var plan subnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subnet, err := r.client.Subnets.ReadSubnet(ctx, &juju.ReadSubnetInput{
		ModelUUID: plan.ModelUUID.ValueString(),
		CIDR:      plan.CIDR.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read subnet before create, got error: %s", err))
		return
	}
	if subnet.SpaceName != alphaSpaceName {
		resp.Diagnostics.AddError(
			"Subnet Not In Alpha Space",
			fmt.Sprintf(
				"Subnet %q is currently in space %q and cannot be managed by juju_subnet create. Move it to %q first or import it.",
				plan.CIDR.ValueString(),
				subnet.SpaceName,
				alphaSpaceName,
			),
		)
		return
	}

	if err := r.client.Spaces.MoveSubnetToSpace(ctx, &juju.MoveSubnetToSpaceInput{
		ModelUUID: plan.ModelUUID.ValueString(),
		SpaceName: plan.SpaceName.ValueString(),
		CIDR:      plan.CIDR.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create subnet resource, got error: %s", err))
		return
	}

	if _, err := wait.WaitFor(wait.WaitForCfg[juju.ReadSubnetInput, *juju.SubnetInfo]{
		Context: ctx,
		GetData: func(ctx context.Context, input juju.ReadSubnetInput) (*juju.SubnetInfo, error) {
			return r.client.Subnets.ReadSubnet(ctx, &input)
		},
		Input: juju.ReadSubnetInput{
			ModelUUID: plan.ModelUUID.ValueString(),
			CIDR:      plan.CIDR.ValueString(),
		},
		DataAssertions: []wait.Assert[*juju.SubnetInfo]{
			func(data *juju.SubnetInfo) error {
				if data.SpaceName != plan.SpaceName.ValueString() {
					return juju.NewRetryReadErrorf("waiting for subnet to be assigned to space %s", data.SpaceName)
				}
				return nil
			},
		},
		NonFatalErrors: []error{errors.NotFound},
		Logf:           r.trace,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("waiting for subnet to move to space %s failed, got error: %s", plan.SpaceName.ValueString(), err))
		return
	}

	plan.ID = types.StringValue(newSubnetResourceID(plan.ModelUUID.ValueString(), plan.CIDR.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	identity := subnetResourceIdentityModel{ID: plan.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Read implements [resource.Resource].
func (r *subnetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "subnet", "read")
		return
	}

	var state subnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	modelUUID := state.ModelUUID.ValueString()
	cidr := state.CIDR.ValueString()
	id := state.ID.ValueString()

	if id != "" {
		parsedModelUUID, parsedCIDR, err := parseSubnetResourceID(id)
		if err != nil {
			resp.Diagnostics.AddError("Malformed ID", err.Error())
			return
		}
		modelUUID = parsedModelUUID
		cidr = parsedCIDR
	}

	if modelUUID == "" || cidr == "" {
		resp.Diagnostics.AddError("Malformed State", "missing subnet model_uuid or cidr in state")
		return
	}

	subnet, err := r.client.Subnets.ReadSubnet(ctx, &juju.ReadSubnetInput{
		ModelUUID: modelUUID,
		CIDR:      cidr,
	})
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read subnet resource, got error: %s", err))
		return
	}

	state.ModelUUID = types.StringValue(modelUUID)
	state.CIDR = types.StringValue(cidr)
	state.SpaceName = types.StringValue(subnet.SpaceName)
	state.ID = types.StringValue(newSubnetResourceID(modelUUID, cidr))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	identity := subnetResourceIdentityModel{ID: state.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Update implements [resource.Resource].
func (r *subnetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "subnet", "update")
		return
	}

	var plan subnetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Spaces.MoveSubnetToSpace(ctx, &juju.MoveSubnetToSpaceInput{
		ModelUUID: plan.ModelUUID.ValueString(),
		SpaceName: plan.SpaceName.ValueString(),
		CIDR:      plan.CIDR.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update subnet resource, got error: %s", err))
		return
	}

	if _, err := wait.WaitFor(wait.WaitForCfg[juju.ReadSubnetInput, *juju.SubnetInfo]{
		Context: ctx,
		GetData: func(ctx context.Context, input juju.ReadSubnetInput) (*juju.SubnetInfo, error) {
			return r.client.Subnets.ReadSubnet(ctx, &input)
		},
		Input: juju.ReadSubnetInput{
			ModelUUID: plan.ModelUUID.ValueString(),
			CIDR:      plan.CIDR.ValueString(),
		},
		DataAssertions: []wait.Assert[*juju.SubnetInfo]{
			func(data *juju.SubnetInfo) error {
				if data.SpaceName != plan.SpaceName.ValueString() {
					return juju.NewRetryReadErrorf("waiting for subnet to be assigned to space %s", plan.SpaceName.ValueString())
				}
				return nil
			},
		},
		NonFatalErrors: []error{errors.NotFound},
		Logf:           r.trace,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("waiting for subnet to move to space %s failed, got error: %s", plan.SpaceName.ValueString(), err))
		return
	}

	plan.ID = types.StringValue(newSubnetResourceID(plan.ModelUUID.ValueString(), plan.CIDR.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	identity := subnetResourceIdentityModel{ID: plan.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Delete implements [resource.Resource].
func (r *subnetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "subnet", "delete")
		return
	}

	var state subnetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subnet, err := r.client.Subnets.ReadSubnet(ctx, &juju.ReadSubnetInput{
		ModelUUID: state.ModelUUID.ValueString(),
		CIDR:      state.CIDR.ValueString(),
	})
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read subnet before delete, got error: %s", err))
		return
	}

	if subnet.SpaceName == alphaSpaceName {
		return
	}

	if err := r.client.Spaces.MoveSubnetToSpace(ctx, &juju.MoveSubnetToSpaceInput{
		ModelUUID: state.ModelUUID.ValueString(),
		SpaceName: alphaSpaceName,
		CIDR:      state.CIDR.ValueString(),
	}); err != nil {
		if errors.Is(err, errors.NotFound) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete subnet resource, got error: %s", err))
		return
	}

	if _, err := wait.WaitFor(wait.WaitForCfg[juju.ReadSubnetInput, *juju.SubnetInfo]{
		Context: ctx,
		GetData: func(ctx context.Context, input juju.ReadSubnetInput) (*juju.SubnetInfo, error) {
			return r.client.Subnets.ReadSubnet(ctx, &input)
		},
		Input: juju.ReadSubnetInput{
			ModelUUID: state.ModelUUID.ValueString(),
			CIDR:      state.CIDR.ValueString(),
		},
		DataAssertions: []wait.Assert[*juju.SubnetInfo]{
			func(data *juju.SubnetInfo) error {
				if data.SpaceName != alphaSpaceName {
					return juju.NewRetryReadError("waiting for subnet to be moved back to alpha")
				}
				return nil
			},
		},
		NonFatalErrors: []error{errors.NotFound},
		Logf:           r.trace,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for subnet deletion, got error: %s", err))
		return
	}
}

func newSubnetResourceID(modelUUID, cidr string) string {
	return fmt.Sprintf("%s:%s", modelUUID, cidr)
}

func parseSubnetResourceID(id string) (string, string, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import identifier with format: <model uuid>:<cidr>. got: %q", id)
	}
	return parts[0], parts[1], nil
}

func (r *subnetResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(r.subCtx, LogResourceSubnet, msg, additionalFields...)
}
