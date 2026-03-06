// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasControllerResource{}
var _ resource.ResourceWithConfigure = &jaasControllerResource{}
var _ resource.ResourceWithImportState = &jaasControllerResource{}
var _ resource.ResourceWithConfigValidators = &jaasControllerResource{}

// NewJAASControllerResource returns a new instance of the JAAS controller resource.
func NewJAASControllerResource() resource.Resource {
	return &jaasControllerResource{}
}

type jaasControllerResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem.
	subCtx context.Context
}

type jaasControllerResourceModel struct {
	Name          types.String `tfsdk:"name"`
	UUID          types.String `tfsdk:"uuid"`
	PublicAddress types.String `tfsdk:"public_address"`
	TLSHostname   types.String `tfsdk:"tls_hostname"`
	APIAddresses  types.List   `tfsdk:"api_addresses"`
	CACertificate types.String `tfsdk:"ca_certificate"`
	Username      types.String `tfsdk:"username"`
	Password      types.String `tfsdk:"password"`

	// Read-only fields returned by JAAS.
	Status types.String `tfsdk:"status"`

	// Delete behavior.
	Force types.Bool `tfsdk:"force"`

	// ID required for imports.
	ID types.String `tfsdk:"id"`
}

// Metadata returns the resource type name.
func (r *jaasControllerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_controller"
}

// Schema defines the schema for the resource.
func (r *jaasControllerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a controller registered in JAAS (JIMM).",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "Name of the controller to register.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"uuid": schema.StringAttribute{
				Description: "UUID of the controller.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"public_address": schema.StringAttribute{
				Description: "Public address of the controller (typically host:port) to be used instead of providing api_addresses.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"tls_hostname": schema.StringAttribute{
				Description: "Hostname used for TLS verification.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			// api_addresses are intentionally a list instead of a set
			// to match the same field in the juju_controller resource.
			"api_addresses": schema.ListAttribute{
				Description: "API addresses of the controller. If the controller is HA, only 1 address needs to be provided" +
					" but multiple addresses are also accepted.",
				Optional:    true,
				ElementType: types.StringType,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"ca_certificate": schema.StringAttribute{
				Description: "CA certificate for the controller.",
				Optional:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"username": schema.StringAttribute{
				Description: "Username that JIMM should use to connect to the controller.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"password": schema.StringAttribute{
				Description: "Password that JIMM should use to connect to the controller.",
				Required:    true,
				Sensitive:   true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"status": schema.StringAttribute{
				Description: "Status of the controller (available/deprecated/unavailable).",
				Computed:    true,
			},
			"force": schema.BoolAttribute{
				Description: "Force removal when deleting (only required when the controller is still available).",
				Optional:    true,
			},
			"id": schema.StringAttribute{
				Description:   "The ID of this resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

// Configure sets the provider configured client to the resource.
func (r *jaasControllerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceJAASController)
}

// ConfigValidators sets validators for the resource.
func (r *jaasControllerResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		NewResourceRequiresJAASValidator(r.client),
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("public_address"),
			path.MatchRoot("api_addresses"),
		),
	}
}

// Create registers the controller with JAAS.
func (r *jaasControllerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "resource-jaas-controller", "create")
		return
	}

	var plan jaasControllerResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Trace(r.subCtx, "creating JAAS controller")

	apiAddrs := make([]string, 0)
	if !plan.APIAddresses.IsNull() && !plan.APIAddresses.IsUnknown() {
		resp.Diagnostics.Append(plan.APIAddresses.ElementsAs(ctx, &apiAddrs, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	info, err := r.client.Jaas.AddController(&params.AddControllerRequest{
		UUID:          plan.UUID.ValueString(),
		Name:          plan.Name.ValueString(),
		PublicAddress: plan.PublicAddress.ValueString(),
		TLSHostname:   plan.TLSHostname.ValueString(),
		APIAddresses:  apiAddrs,
		CACertificate: plan.CACertificate.ValueString(),
		Username:      plan.Username.ValueString(),
		Password:      plan.Password.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add JAAS controller %q, got error: %s", plan.Name.ValueString(), err))
		return
	}

	state := plan
	state.Name = types.StringValue(info.Name)
	state.UUID = types.StringValue(info.UUID)
	state.PublicAddress = types.StringValue(info.PublicAddress)
	// Avoid syncing the remote API addresses to prevent conflict if the controller becomes HA or simply has multiple addresses reported by JIMM.
	state.APIAddresses = plan.APIAddresses
	state.CACertificate = types.StringValue(info.CACertificate)
	state.Status = types.StringValue(string(info.Status.Status))
	// Keep the password/username from the plan.
	state.Username = plan.Username
	state.Password = plan.Password
	state.ID = types.StringValue(info.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Read refreshes the state of the resource from the remote JAAS API.
func (r *jaasControllerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "resource-jaas-controller", "read")
		return
	}

	var state jaasControllerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	controllers, err := r.client.Jaas.ListControllers()
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list JAAS controllers, got error: %s", err))
		return
	}

	var found *params.ControllerInfo
	for i := range controllers {
		if controllers[i].Name == state.ID.ValueString() {
			found = &controllers[i]
			break
		}
	}
	if found == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	state.Name = types.StringValue(found.Name)
	state.UUID = types.StringValue(found.UUID)
	state.PublicAddress = types.StringValue(found.PublicAddress)
	// Avoid syncing the remote API addresses to prevent conflict if the controller becomes HA or simply has multiple addresses reported by JIMM.
	// state.APIAddresses, _ = types.ListValueFrom(ctx, types.StringType, found.APIAddresses)
	state.CACertificate = types.StringValue(found.CACertificate)
	state.Status = types.StringValue(string(found.Status.Status))
	// Do not overwrite sensitive fields (username/password) on Read.
	state.ID = types.StringValue(found.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

// Update is not supported for this resource since all fields require replacement.
func (r *jaasControllerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unsupported", "juju_jaas_controller does not support update; changes require replacement")
}

// Delete removes the controller from JAAS.
func (r *jaasControllerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "resource-jaas-controller", "delete")
		return
	}

	var state jaasControllerResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Jaas.RemoveController(&params.RemoveControllerRequest{Name: state.Name.ValueString(), Force: state.Force.ValueBool()})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove JAAS controller %q, got error: %s", state.Name.ValueString(), err))
		return
	}
}

// ImportState imports the resource by its name, which is also used as the ID.
func (r *jaasControllerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
