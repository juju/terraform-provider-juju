// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ ephemeral.EphemeralResourceWithConfigure = &externalControllerEphemeral{}

// NewExternalControllerEphemeral returns a new ephemeral resource for registering an external Juju Controller.
func NewExternalControllerEphemeral() ephemeral.EphemeralResourceWithConfigure {
	return &externalControllerEphemeral{}
}

type externalControllerEphemeral struct {
	client *juju.Client

	// context for the logging subsystem.
	subCtx context.Context
}

type externalControllerEphemeralModel struct {
	ControllerName  types.String `tfsdk:"controller_name"`
	ControllerAddrs types.String `tfsdk:"controller_addresses"`
	UserName        types.String `tfsdk:"username"`
	Password        types.String `tfsdk:"password"`
	CACert          types.String `tfsdk:"ca_certificate"`
}

// Metadata returns the full ephemeral resource name as used in terraform plans.
func (e *externalControllerEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_external_controller"
}

// Schema returns the schema for the external controller ephemeral resource.
func (e *externalControllerEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Registers an external Juju controller configuration for cross-model operations. " +
			"This ephemeral resource executes during terraform apply to configure external controller access.",
		Attributes: map[string]schema.Attribute{
			"controller_name": schema.StringAttribute{
				Description: "The name of the external controller to register.",
				Required:    true,
			},
			"controller_addresses": schema.StringAttribute{
				Description: fmt.Sprintf("Controller addresses to connect to. Multiple addresses can be provided in this format: <host>:<port>,<host>:<port>,.... This can also be set by the `%s` environment variable.", JujuControllerEnvKey),
				Required:    true,
			},
			"username": schema.StringAttribute{
				Description: fmt.Sprintf("Username registered with the controller. This can also be set by the `%s` environment variable.", JujuUsernameEnvKey),
				Required:    true,
			},
			"password": schema.StringAttribute{
				Description: fmt.Sprintf("Password for the controller username. This can also be set by the `%s` environment variable.", JujuPasswordEnvKey),
				Sensitive:   true,
				Required:    true,
			},
			"ca_certificate": schema.StringAttribute{
				Description: fmt.Sprintf("CA certificate for the controller if using a self-signed certificate. This can also be set by the `%s` environment variable.", JujuCACertEnvKey),
				Optional:    true,
			},
		},
	}
}

// Configure enables provider-level data or clients to be set in the
// provider-defined ephemeral resource type.
func (e *externalControllerEphemeral) Configure(ctx context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	e.client = provider.Client
	e.subCtx = tflog.NewSubsystem(ctx, "ephemeral_external_controller")
}

// Open is called when the ephemeral resource is opened during terraform apply.
// This is where the side effect of registering the external controller occurs.
func (e *externalControllerEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	// Prevent panic if the provider has not been configured.
	if e.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured Client",
			"The provider client was not configured. Please report this issue to the provider developers.",
		)
		return
	}

	var config externalControllerEphemeralModel

	// Read Terraform configuration into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Register the external controller configuration
	tflog.Info(ctx, "Registering external controller configuration", map[string]interface{}{
		"controller_name": config.ControllerName.ValueString(),
	})

	err := e.client.Applications.AddExternalControllerConf(
		config.ControllerName.ValueString(),
		juju.ControllerConfiguration{
			ControllerAddresses: strings.Split(config.ControllerAddrs.ValueString(), ","),
			Username:            config.UserName.ValueString(),
			Password:            config.Password.ValueString(),
			CACert:              config.CACert.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error Registering External Controller Configuration",
			fmt.Sprintf("An error was encountered while registering the external controller configuration: %s", err.Error()),
		)
		return
	}

	tflog.Info(ctx, "Successfully registered external controller configuration", map[string]interface{}{
		"controller_name": config.ControllerName.ValueString(),
	})

	// Set the result - ephemeral resources can return data but don't persist to state
	resp.Diagnostics.Append(resp.Result.Set(ctx, &config)...)
}

// Close is called when the ephemeral resource is closed.
// This can be used for cleanup operations if needed.
func (e *externalControllerEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	tflog.Debug(ctx, "External controller ephemeral resource closed")
	// Optional: Add cleanup logic here if needed
	// For example, you might want to remove the controller configuration
}
