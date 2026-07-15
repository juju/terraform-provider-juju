// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

var _ resource.Resource = &secretBackendResource{}
var _ resource.ResourceWithConfigure = &secretBackendResource{}
var _ resource.ResourceWithImportState = &secretBackendResource{}
var _ resource.ResourceWithIdentity = &secretBackendResource{}
var _ resource.ResourceWithConfigValidators = &secretBackendResource{}

// NewSecretBackendResource returns a new instance of the secret backend resource.
func NewSecretBackendResource() resource.Resource {
	return &secretBackendResource{}
}

type secretBackendResource struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type secretBackendResourceModel struct {
	Name                types.String `tfsdk:"name"`
	BackendType         types.String `tfsdk:"backend_type"`
	TokenRotateInterval types.String `tfsdk:"token_rotate_interval"`
	// ConfigWO is the write-only backend configuration. Its content is never
	// persisted to Terraform state; it is read from the configuration only.
	ConfigWO types.Map `tfsdk:"config_wo"`
	// ConfigWOVersion triggers an update of the write-only ConfigWO. Bump this
	// whenever the underlying write-only value changes.
	ConfigWOVersion types.Int64 `tfsdk:"config_wo_version"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

type secretBackendResourceIdentityModel struct {
	Name types.String `tfsdk:"name"`
}

// Configure implements resource.ResourceWithConfigure.
func (r *secretBackendResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.client = provider.Client
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceSecretBackend)
}

// Metadata implements resource.Resource.
func (r *secretBackendResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret_backend"
}

// ConfigValidators implements [resource.ResourceWithConfigValidators]. It
// enforces that config_wo and config_wo_version are both set.
func (r *secretBackendResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.RequiredTogether(
			path.MatchRoot("config_wo"),
			path.MatchRoot("config_wo_version"),
		),
	}
}

// Schema implements resource.Resource.
func (r *secretBackendResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: `A resource that represents a Juju secret backend.

Secret backends store secret content. To learn more about secret backends, please visit: https://documentation.ubuntu.com/juju/3.6/reference/secret-backends/
		`,
		Attributes: map[string]schema.Attribute{
			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Description: "The name of the secret backend.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"backend_type": schema.StringAttribute{
				Description: "The type of the secret backend (e.g., 'vault', 'kubernetes').",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"token_rotate_interval": schema.StringAttribute{
				Description: "The interval at which the backend's access credential/token should be rotated. " +
					"Must be a duration string parseable by Go's time.ParseDuration (e.g., '10m', '1h', '24h').",
				Optional: true,
			},
			"config_wo": schema.MapAttribute{
				Description: "The write-only backend configuration. Its content is never persisted to" +
					" Terraform state. Requires config_wo_version to be set; bump config_wo_version to" +
					" apply changes to this value.",
				ElementType: types.StringType,
				Required:    true,
				WriteOnly:   true,
				Sensitive:   true,
			},
			"config_wo_version": schema.Int64Attribute{
				Description: "The version of config_wo. Increment this value to trigger an update of the" +
					" write-only backend configuration.",
				Required: true,
			},
		},
	}
}

// IdentitySchema implements [resource.ResourceWithIdentity].
func (r *secretBackendResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = identityschema.Schema{
		Attributes: map[string]identityschema.Attribute{
			"name": identityschema.StringAttribute{
				RequiredForImport: true,
			},
		},
	}
}

func (r *secretBackendResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	name := ""

	if req.ID != "" {
		name = req.ID
	} else {
		var identityData secretBackendResourceIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identityData)...)
		if resp.Diagnostics.HasError() {
			return
		}

		name = identityData.Name.ValueString()
	}

	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secretbackend", "import")
		return
	}

	if name == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			"Expected import identifier with format: <secret backend name>. Got an empty string.",
		)
		return
	}

	state := secretBackendResourceModel{
		Name:     types.StringValue(name),
		ConfigWO: types.MapNull(types.StringType),
	}
	state.ID = generateSecretBackendResourceID(state)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	identity := secretBackendResourceIdentityModel{Name: state.Name}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Create implements resource.Resource.
func (r *secretBackendResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secretbackend", "create")
		return
	}

	var plan secretBackendResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configAny, err := r.resolveConfig(ctx, plan, req.Config)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get secret backend config as map, got error: %s", err))
		return
	}

	var tokenRotateInterval *time.Duration
	if !plan.TokenRotateInterval.IsNull() && !plan.TokenRotateInterval.IsUnknown() {
		d, err := time.ParseDuration(plan.TokenRotateInterval.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse token_rotate_interval, got error: %s", err))
			return
		}
		tokenRotateInterval = &d
	}

	if err := r.client.SecretBackends.CreateSecretBackend(
		ctx,
		juju.CreateSecretBackendInput{
			Name:                plan.Name.ValueString(),
			BackendType:         plan.BackendType.ValueString(),
			TokenRotateInterval: tokenRotateInterval,
			Config:              configAny,
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create secret backend resource, got error: %s", err))
		return
	}

	r.trace("created secret backend", map[string]interface{}{
		"name":         plan.Name.ValueString(),
		"backend_type": plan.BackendType.ValueString(),
	})

	plan.ID = generateSecretBackendResourceID(plan)
	// config_wo is write-only and never read back; keep it a typed null
	// so it does not appear in state.
	plan.ConfigWO = types.MapNull(types.StringType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	identity := secretBackendResourceIdentityModel{Name: plan.Name}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Read implements resource.Resource.
func (r *secretBackendResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secretbackend", "read")
		return
	}

	var state secretBackendResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	getBackendResp, err := r.client.SecretBackends.GetSecretBackend(
		ctx,
		juju.GetSecretBackendInput{
			Name: state.Name.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret backend resource, got error: %s", err))
		return
	}
	r.trace(fmt.Sprintf("read secret backend resource %q", getBackendResp.Backend.Name))

	state.Name = types.StringValue(getBackendResp.Backend.Name)
	state.BackendType = types.StringValue(getBackendResp.Backend.BackendType)
	if getBackendResp.Backend.TokenRotateInterval != nil {
		state.TokenRotateInterval = types.StringValue(getBackendResp.Backend.TokenRotateInterval.String())
	} else {
		state.TokenRotateInterval = types.StringNull()
	}
	// config_wo is write-only and never read back; keep it a typed null.
	state.ConfigWO = types.MapNull(types.StringType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	identity := secretBackendResourceIdentityModel{Name: state.Name}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Update implements resource.Resource.
func (r *secretBackendResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secretbackend", "update")
		return
	}

	var plan, state secretBackendResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configAny, err := r.resolveConfig(ctx, plan, req.Config)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get secret backend config as map, got error: %s", err))
		return
	}

	var tokenRotateInterval *time.Duration
	if !plan.TokenRotateInterval.IsNull() && !plan.TokenRotateInterval.IsUnknown() {
		d, err := time.ParseDuration(plan.TokenRotateInterval.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse token_rotate_interval, got error: %s", err))
			return
		}
		tokenRotateInterval = &d
	}

	// Determine if the name has changed.
	var nameChange *string
	if plan.Name.ValueString() != state.Name.ValueString() {
		newName := plan.Name.ValueString()
		nameChange = &newName
	}

	if err := r.client.SecretBackends.UpdateSecretBackend(ctx,
		juju.UpdateSecretBackendInput{
			Name:                state.Name.ValueString(),
			NameChange:          nameChange,
			TokenRotateInterval: tokenRotateInterval,
			Config:              configAny,
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update secret backend resource, got error: %s", err))
		return
	}

	r.trace("updated secret backend", map[string]interface{}{
		"name":         plan.Name.ValueString(),
		"backend_type": plan.BackendType.ValueString(),
	})

	plan.ID = generateSecretBackendResourceID(plan)
	// config_wo is write-only and never read back; keep it a typed null
	// so it does not appear in state.
	plan.ConfigWO = types.MapNull(types.StringType)

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	identity := secretBackendResourceIdentityModel{Name: plan.Name}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

// Delete implements resource.Resource.
func (r *secretBackendResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secretbackend", "delete")
		return
	}

	var state secretBackendResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.SecretBackends.RemoveSecretBackend(
		ctx,
		juju.RemoveSecretBackendInput{
			Name: state.Name.ValueString(),
		},
	); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete secret backend resource, got error: %s", err))
		return
	}
	r.trace("deleted secret backend", map[string]interface{}{
		"name": state.Name.ValueString(),
	})
}

func (s *secretBackendResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(s.subCtx, LogResourceSecretBackend, msg, additionalFields...)
}

// resolveConfig returns the backend config to send to Juju, reading from the
// write-only config_wo attribute (via config) when config_wo_version is set.
func (r *secretBackendResource) resolveConfig(
	ctx context.Context,
	plan secretBackendResourceModel,
	config tfsdk.Config,
) (map[string]any, error) {
	if !plan.ConfigWOVersion.IsNull() {
		var configWO map[string]string
		diags := config.GetAttribute(ctx, path.Root("config_wo"), &configWO)
		if diags.HasError() {
			return nil, fmt.Errorf("failed to read config_wo: %v", diags)
		}
		result := make(map[string]any, len(configWO))
		for k, v := range configWO {
			result[k] = v
		}
		return result, nil
	}

	return nil, nil
}

func generateSecretBackendResourceID(plan secretBackendResourceModel) basetypes.StringValue {
	return types.StringValue(plan.Name.ValueString())
}
