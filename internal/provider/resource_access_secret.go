// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/collections/set"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &accessSecretResource{}
var _ resource.ResourceWithConfigure = &accessSecretResource{}
var _ resource.ResourceWithImportState = &accessSecretResource{}

func NewAccessSecretResource() resource.Resource {
	return &accessSecretResource{}
}

type accessSecretResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type accessSecretResourceModel struct {
	// Model to which the secret belongs.
	Model types.String `tfsdk:"model"`
	// SecretId is the ID of the secret to be grant or revoked.
	SecretId types.String `tfsdk:"secret_id"`

	// ID is used during terraform import.
	ID types.String `tfsdk:"id"`
}

type accessSecretResourceModelV0 struct {
	accessSecretResourceModel

	// Applications is a list of applications to which the secret is granted or revoked.
	Applications types.List `tfsdk:"applications"`
}

type accessSecretResourceModelV1 struct {
	accessSecretResourceModel

	// Applications is a set of applications to which the secret is granted or revoked.
	Applications types.Set `tfsdk:"applications"`
}

// ImportState reads the secret based on the model name and secret name to be
// imported into terraform.
func (s *accessSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access secret", "import")
		return
	}
	// model:name
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: <modelname>:<secretname>. Got: %q", req.ID),
		)
		return
	}
	modelName := parts[0]
	secretName := parts[1]

	readSecretOutput, err := s.client.Secrets.ReadSecret(&juju.ReadSecretInput{
		ModelName: modelName,
		Name:      &secretName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret for import, got error: %s", err))
		return
	}

	// Save the secret access details into the Terraform state
	state := accessSecretResourceModelV1{
		accessSecretResourceModel: accessSecretResourceModel{
			Model:    types.StringValue(modelName),
			SecretId: types.StringValue(readSecretOutput.SecretId),
			ID:       types.StringValue(newSecretID(modelName, readSecretOutput.SecretId)),
		},
	}

	// Save the secret details into the Terraform state
	secretApplications, errDiag := types.SetValueFrom(ctx, types.StringType, readSecretOutput.Applications)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Applications = secretApplications

	// Save state into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("import access secret resource %q", state.SecretId))
}

func (s *accessSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_access_secret"
}

// Schema is called when the resource schema is being initialized.
func (s *accessSecretResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represents a Juju secret access.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The model in which the secret belongs.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"secret_id": schema.StringAttribute{
				Description: "The ID of the secret. E.g. coj8mulh8b41e8nv6p90",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"applications": schema.SetAttribute{
				Description: "The list of applications to which the secret is granted.",
				Required:    true,
				ElementType: types.StringType,
			},
			"id": schema.StringAttribute{
				Description: "The ID of the secret. Used for terraform import.",
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
		Version: 1,
	}
}

// Configure is called when the resource is being configured.
func (s *accessSecretResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, ok := req.ProviderData.(juju.ProviderData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected juju.ProviderData, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	s.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	s.subCtx = tflog.NewSubsystem(ctx, LogResourceAccessSecret)
}

// Create is called when the resource is being created.
func (s *accessSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "secret", "create")
		return
	}

	var plan accessSecretResourceModelV1

	// Read Terraform plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	applications := make([]string, len(plan.Applications.Elements()))
	resp.Diagnostics.Append(plan.Applications.ElementsAs(ctx, &applications, false)...)

	err := s.client.Secrets.UpdateAccessSecret(&juju.GrantRevokeAccessSecretInput{
		ModelName:    plan.Model.ValueString(),
		SecretId:     plan.SecretId.ValueString(),
		Applications: applications,
	}, juju.GrantAccess)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to grant secret access, got error: %s", err))
		return
	}

	// Save plan into Terraform state
	plan.ID = types.StringValue(newSecretID(plan.Model.ValueString(), plan.SecretId.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	s.trace(fmt.Sprintf("grant secret access to %s", plan.SecretId))
}

// Read is called when the resource is being read.
func (s *accessSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access_secret", "read")
		return
	}

	var state accessSecretResourceModelV1

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	readSecretOutput, err := s.client.Secrets.ReadSecret(&juju.ReadSecretInput{
		SecretId:  state.SecretId.ValueString(),
		ModelName: state.Model.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read secret, got error: %s", err))
		return
	}

	// Save the secret details into the Terraform state
	secretApplications, errDiag := types.SetValueFrom(ctx, types.StringType, readSecretOutput.Applications)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Applications = secretApplications

	state.ID = types.StringValue(newSecretID(state.Model.ValueString(), readSecretOutput.SecretId))

	// Save state into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("read secret access %s", state.SecretId))
}

// Update is called when the resource is being updated.
func (s *accessSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access_secret", "update")
		return
	}

	var plan, state accessSecretResourceModelV1

	// Read Terraform plan and state into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var updatedAccessSecretInput juju.GrantRevokeAccessSecretInput

	updatedAccessSecretInput.ModelName = state.Model.ValueString()
	updatedAccessSecretInput.SecretId = state.SecretId.ValueString()

	if plan.Applications.Equal(state.Applications) {
		s.trace(fmt.Sprintf("no updates to secret access %q", state.SecretId))
		return
	}

	planApplications := make([]string, len(plan.Applications.Elements()))
	resp.Diagnostics.Append(plan.Applications.ElementsAs(ctx, &planApplications, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateApplications := make([]string, len(state.Applications.Elements()))
	resp.Diagnostics.Append(state.Applications.ElementsAs(ctx, &stateApplications, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planSet := set.NewStrings(planApplications...)
	stateSet := set.NewStrings(stateApplications...)

	applicationsToGrant := planSet.Difference(stateSet)
	applicationsToRevoke := stateSet.Difference(planSet)

	s.trace(fmt.Sprintf("applications to revoke secret: %v", applicationsToRevoke))
	s.trace(fmt.Sprintf("applications to grant secret: %v", applicationsToGrant))

	resp.Diagnostics.Append(plan.Applications.ElementsAs(ctx, &state.Applications, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(plan.Applications.ElementsAs(ctx, &updatedAccessSecretInput.Applications, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// revoke access to applications that are in the state but not in the plan
	if !applicationsToGrant.IsEmpty() {
		err := s.client.Secrets.UpdateAccessSecret(&juju.GrantRevokeAccessSecretInput{
			ModelName:    state.Model.ValueString(),
			SecretId:     state.SecretId.ValueString(),
			Applications: applicationsToGrant.Values(),
		}, juju.GrantAccess)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to grant secret access, got error: %s", err))
			return
		}
	}

	// grant access to applications that are in the plan but not in the state
	if !applicationsToRevoke.IsEmpty() {
		err := s.client.Secrets.UpdateAccessSecret(&juju.GrantRevokeAccessSecretInput{
			ModelName:    state.Model.ValueString(),
			SecretId:     state.SecretId.ValueString(),
			Applications: applicationsToRevoke.Values(),
		}, juju.RevokeAccess)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to revoke secret access, got error: %s", err))
			return
		}
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("update secret access %s", state.SecretId))
}

// Delete is called when the resource is being deleted.
func (s *accessSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if s.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "access_secret", "delete")
		return
	}

	var state accessSecretResourceModelV1

	// Read Terraform configuration state into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	applications := make([]string, len(state.Applications.Elements()))
	resp.Diagnostics.Append(state.Applications.ElementsAs(ctx, &applications, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := s.client.Secrets.UpdateAccessSecret(&juju.GrantRevokeAccessSecretInput{
		ModelName:    state.Model.ValueString(),
		SecretId:     state.SecretId.ValueString(),
		Applications: applications,
	}, juju.RevokeAccess)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to revoke secret access, got error: %s", err))
		return
	}

	// Save empty set of applications into Terraform state
	emptyApplicationSet, errDiag := types.SetValue(types.StringType, []attr.Value{})
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Applications = emptyApplicationSet

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	s.trace(fmt.Sprintf("revoke secret access %s", state.SecretId))
}

func (s *accessSecretResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if s.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(s.subCtx, LogResourceAccessSecret, msg, additionalFields...)
}

// UpgradeState upgrades the state of the juju_access_secret resource.
// This is used to handle changes in the resource schema between versions.
func (o *accessSecretResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		// Upgrade from List to Set for `applications`.
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"model": schema.StringAttribute{
						Required: true,
					},
					"secret_id": schema.StringAttribute{
						Required: true,
					},
					"applications": schema.ListAttribute{
						Required:    true,
						ElementType: types.StringType,
					},
					"id": schema.StringAttribute{
						Computed: true,
					},
				},
				Version: 0,
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				var priorStateData accessSecretResourceModelV0

				resp.Diagnostics.Append(req.State.Get(ctx, &priorStateData)...)
				if resp.Diagnostics.HasError() {
					return
				}

				applications := []string{}
				if !priorStateData.Applications.IsNull() {
					resp.Diagnostics.Append(priorStateData.Applications.ElementsAs(ctx, &applications, false)...)
				}

				applicationsSet, diags := types.SetValueFrom(ctx, types.StringType, applications)
				if diags.HasError() {
					resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to convert applications to set, got error: %s", diags))
					return
				}

				upgradedStateData := accessSecretResourceModelV1{
					accessSecretResourceModel: accessSecretResourceModel{
						Model:    priorStateData.Model,
						SecretId: priorStateData.SecretId,
						ID:       priorStateData.ID,
					},
					Applications: applicationsSet,
				}

				resp.Diagnostics.Append(resp.State.Set(ctx, upgradedStateData)...)
			},
		},
	}
}
