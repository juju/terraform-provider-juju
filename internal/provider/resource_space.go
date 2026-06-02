// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

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

const alphaSpaceName = "alpha"
const systemSpaceNotManageableMsg = "alpha is a system space and cannot be managed by juju_space"

var _ resource.Resource = &spaceResource{}
var _ resource.ResourceWithConfigure = &spaceResource{}
var _ resource.ResourceWithImportState = &spaceResource{}
var _ resource.ResourceWithIdentity = &spaceResource{}

// NewSpaceResource returns a new space resource.
func NewSpaceResource() resource.Resource {
	return &spaceResource{}
}

type spaceResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type spaceResourceModel struct {
	ModelUUID types.String `tfsdk:"model_uuid"`
	Name      types.String `tfsdk:"name"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type spaceResourceIdentityModel struct {
	ID types.String `tfsdk:"id"`
}

func (r *spaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (r *spaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju space.",
		Attributes: map[string]schema.Attribute{
			"model_uuid": schema.StringAttribute{
				Description: "The UUID of the model where the space belongs.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidModel, "must be a valid UUID"),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the space. Changing this value forces replacement.",
				Required:    true,
				Validators: []validator.String{
					ValidatorMatchString(names.IsValidSpace, "must be a valid space name"),
					ValidatorMatchString(func(name string) bool {
						return !isSystemSpace(name)
					}, systemSpaceNotManageableMsg),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"id": schema.StringAttribute{
				Description: "The identifier of the space resource. Format: <model_uuid>:<name>",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

// IdentitySchema implements [resource.ResourceWithIdentity].
func (r *spaceResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"id": identityschema.StringAttribute{
				RequiredForImport: true,
			},
		},
	}
}

func (r *spaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSpace)
}

func (r *spaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idStr := ""
	if req.ID != "" {
		idStr = req.ID
	} else {
		var identityData spaceResourceIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identityData)...)
		if resp.Diagnostics.HasError() {
			return
		}
		idStr = identityData.ID.ValueString()
	}

	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "space", "import")
		return
	}

	modelUUID, spaceName, err := parseSpaceResourceID(idStr)
	if err != nil {
		resp.Diagnostics.AddError("Unexpected Import Identifier", err.Error())
		return
	}
	if isSystemSpace(spaceName) {
		resp.Diagnostics.AddError("System Space Not Manageable", systemSpaceNotManageableMsg)
		return
	}

	state := spaceResourceModel{
		ModelUUID: types.StringValue(modelUUID),
		Name:      types.StringValue(spaceName),
		ID:        types.StringValue(newSpaceResourceID(modelUUID, spaceName)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	identity := spaceResourceIdentityModel{ID: state.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

func (r *spaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "space", "create")
		return
	}

	var plan spaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Spaces.CreateSpace(ctx, &juju.CreateSpaceInput{
		ModelUUID: plan.ModelUUID.ValueString(),
		Name:      plan.Name.ValueString(),
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create space resource, got error: %s", err))
		return
	}

	if _, err := wait.WaitFor(wait.WaitForCfg[juju.ReadSpaceInput, *juju.ReadSpaceOutput]{
		Context: ctx,
		GetData: func(ctx context.Context, input juju.ReadSpaceInput) (*juju.ReadSpaceOutput, error) {
			return r.client.Spaces.ReadSpace(ctx, &input)
		},
		Input: juju.ReadSpaceInput{
			ModelUUID: plan.ModelUUID.ValueString(),
			Name:      plan.Name.ValueString(),
		},
		DataAssertions: []wait.Assert[*juju.ReadSpaceOutput]{
			func(data *juju.ReadSpaceOutput) error {
				if data.Name != plan.Name.ValueString() {
					return juju.NewRetryReadError("waiting for space to be created")
				}
				return nil
			},
		},
		NonFatalErrors: []error{errors.NotFound},
		Logf:           r.trace,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for space, got error: %s", err))
		return
	}

	plan.ID = types.StringValue(newSpaceResourceID(plan.ModelUUID.ValueString(), plan.Name.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	identity := spaceResourceIdentityModel{ID: plan.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

func (r *spaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "space", "read")
		return
	}

	var state spaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Spaces.ReadSpace(ctx, &juju.ReadSpaceInput{
		ModelUUID: state.ModelUUID.ValueString(),
		Name:      state.Name.ValueString(),
	})
	if err != nil {
		if errors.Is(err, errors.NotFound) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read space resource, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	identity := spaceResourceIdentityModel{ID: state.ID}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

func (r *spaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update Not Supported", "space resources cannot be updated. To change the name or model of a space, you must destroy and recreate the resource with the new values.")
}

func (r *spaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "space", "delete")
		return
	}

	var state spaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Spaces.DeleteSpace(ctx, &juju.DeleteSpaceInput{
		ModelUUID: state.ModelUUID.ValueString(),
		Name:      state.Name.ValueString(),
	}); err != nil {
		if errors.Is(err, errors.NotFound) {
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete space resource, got error: %s", err))
		return
	}

	if err := wait.WaitForError(wait.WaitForErrorCfg[juju.ReadSpaceInput, *juju.ReadSpaceOutput]{
		Context: ctx,
		GetData: func(ctx context.Context, input juju.ReadSpaceInput) (*juju.ReadSpaceOutput, error) {
			return r.client.Spaces.ReadSpace(ctx, &input)
		},
		Input: juju.ReadSpaceInput{
			ModelUUID: state.ModelUUID.ValueString(),
			Name:      state.Name.ValueString(),
		},
		ExpectedErr:    errors.NotFound,
		RetryAllErrors: true,
		Logf:           r.trace,
	}); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to wait for space deletion, got error: %s", err))
		return
	}
}

func newSpaceResourceID(modelUUID, name string) string {
	return fmt.Sprintf("%s:%s", modelUUID, name)
}

func parseSpaceResourceID(id string) (string, string, error) {
	parts := strings.Split(id, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("expected import identifier with format: <model uuid>:<space name>. got: %q", id)
	}
	return parts[0], parts[1], nil
}

func isSystemSpace(name string) bool {
	return name == alphaSpaceName
}

func (r *spaceResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(r.subCtx, LogResourceSpace, msg, additionalFields...)
}
