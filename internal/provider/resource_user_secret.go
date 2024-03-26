// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"github.com/google/uuid"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

type userSecretResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type userSecretResourceModel struct {
	// ID of the user to whom the secret is added, updated or from whom it is removed. This attribute is required.
	UserId types.String `tsfsdk:"user_id"`
	// ID of the secret to be updated or removed. This attribute is required for 'update' and 'remove' actions.
	SecretID types.String `tsfsdk:"secret_id"`
	// Action to be performed - either 'add', 'update' or 'remove'. This attribute is required.
	Action types.String `tsfsdk:"action"`
	// Value of the secret to be added or updated. This attribute is required for 'add' and 'update' actions.
	// Template: [<key>[#base64]]=<value>[ ...]
	Value types.String `tsfsdk:"value"`
	// ID is required by the testing framework
	ID types.String `tsfsdk:"id"`
}

func (u *userSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_secret"
}

func (u *userSecretResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju user secret.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Description: "ID of the user to whom the secret is added, updated or from whom it is removed.",
				Required:    true,
			},
			"secret_id": schema.StringAttribute{
				Description: "ID of the secret to be updated or removed.",
			},
			"action": schema.StringAttribute{
				Description: "Action to be performed - either 'add', 'update' or 'remove'.",
				Required:    true,
			},
			"value": schema.StringAttribute{
				Description: "Value of the secret to be added or updated.",
			},
		},
	}
}

func (u *userSecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	u.client = client
	// Create the local logging subsystem here, using the TF context when creating it.
	u.subCtx = tflog.NewSubsystem(ctx, LogResourceUserSecret)
}

func (u *userSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Read the user secret.
	// TODO: Implement this method.
}

func (u *userSecretResource) Add(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if u.client == nil {
		resp.Diagnostics.AddError(
			"Unconfigured HTTP Client",
			"Expected configured HTTP client. Please report this issue to the provider developers.",
		)
		return
	}

	var data userSecretResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data))
	if resp.Diagnostics.HasError() {
		return
	}

	// TODO - implement the UserSecret in internal.
	_, err := u.client.UserSecrets.AddUserSecret(&juju.AddUserSecretInput{
		UserId: data.UserId.ValueString(),
		Value:  data.Value.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add user secret, got error: %s", err))
		return
	}
	u.trace(fmt.Sprintf("add user secret resource %q", data.UserId))

	// Save data into Terraform state
	data.ID = types.StringValue(newID())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data))
}

func (u *userSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Update the user secret.
	// TODO: Implement this method.
}

func (u *userSecretResource) Remove(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Remove the user secret.
	// TODO: Implement this method.
}

func (u *userSecretResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if u.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(u.subCtx, LogResourceUserSecret, msg, additionalFields...)
}

// ID is 'user-secret:<random-uuid>'
func newID() string {
	return fmt.Sprintf("user-secret:%s", uuid.New().String())
}
