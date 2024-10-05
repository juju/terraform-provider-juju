// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"strings"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &kubernetesCloudResource{}
var _ resource.ResourceWithConfigure = &kubernetesCloudResource{}
var _ resource.ResourceWithImportState = &kubernetesCloudResource{}

func NewKubernetesCloudResource() resource.Resource {
	return &kubernetesCloudResource{}
}

type kubernetesCloudResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type kubernetesCloudResourceModel struct {
	CloudName         types.String `tfsdk:"name"`
	CloudCredential   types.String `tfsdk:"credential"`
	KubernetesConfig  types.String `tfsdk:"kubernetes_config"`
	ParentCloudName   types.String `tfsdk:"parent_cloud_name"`
	ParentCloudRegion types.String `tfsdk:"parent_cloud_region"`
	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

// Configure is used to configure the kubernetes cloud resource.
func (r *kubernetesCloudResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceKubernetesCloud)
}

// ImportState is used to import kubernetes cloud into Terraform.
func (r *kubernetesCloudResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes-cloud", "import")
		return
	}

	// cloud-name:cloud-credential-name
	parts := strings.Split(req.ID, ":")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError("Invalid ID", fmt.Sprintf("Invalid ID %q, expected format cloud-name:cloud-credential-name", req.ID))
		return
	}
	cloudName := parts[0]

	readKubernetesCloudOutput, err := r.client.Clouds.ReadKubernetesCloud(
		juju.ReadKubernetesCloudInput{
			Name: cloudName,
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read kubernetes readKubernetesCloudOutput, got error %s", err))
		return
	}

	// Set the state onto the Terraform state
	state := kubernetesCloudResourceModel{
		CloudName:         types.StringValue(readKubernetesCloudOutput.Name),
		CloudCredential:   types.StringValue(readKubernetesCloudOutput.CredentialName),
		ParentCloudName:   types.StringValue(readKubernetesCloudOutput.ParentCloudName),
		ParentCloudRegion: types.StringValue(readKubernetesCloudOutput.ParentCloudRegion),
	}

	// Save the state to the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)

	r.trace(fmt.Sprintf("Imported kubernetes cloud %s", cloudName))
}

// Metadata returns the metadata for the kubernetes cloud resource.
func (r *kubernetesCloudResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cloud"
}

// Schema returns the schema for the kubernetes cloud resource.
func (r *kubernetesCloudResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Cloud for existing controller.",
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Description: "The name of the cloud. Changing this value will cause the cloud to be destroyed and recreated by terraform.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credential": schema.StringAttribute{
				Description: "The name of the credential created for this cloud.",
				Computed:    true,
			},
			"kubernetes_config": schema.StringAttribute{
				Description: "The kubernetes config file path for the cloud. Cloud credentials will be added to the Juju controller for you.",
				Optional:    true,
				Sensitive:   true,
			},
			"parent_cloud_name": schema.StringAttribute{
				Description: "The parent cloud name in case adding k8s cluster from existed cloud. Changing this value will cause the cloud to be destroyed and recreated by terraform.",
				Optional:    true,
			},
			"parent_cloud_region": schema.StringAttribute{
				Description: "The parent cloud region name in case adding k8s cluster from existed cloud. Changing this value will cause the cloud to be destroyed and recreated by terraform.",
				Optional:    true,
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

// Create adds a new kubernetes cloud to controllers used now by Terraform provider.
func (r *kubernetesCloudResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes_cloud", "create")
		return
	}

	var plan kubernetesCloudResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create the kubernetes cloud.
	cloudCredentialName, err := r.client.Clouds.CreateKubernetesCloud(
		&juju.CreateKubernetesCloudInput{
			Name:             plan.CloudName.ValueString(),
			KubernetesConfig: plan.KubernetesConfig.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create kubernetes cloud, got error %s", err))
		return
	}

	r.trace(fmt.Sprintf("Created kubernetes cloud %s", plan.CloudName.ValueString()))

	plan.CloudCredential = types.StringValue(cloudCredentialName)
	plan.ID = types.StringValue(newKubernetesCloudID(plan.CloudName.ValueString(), plan.CloudCredential.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read reads the current state of the kubernetes cloud.
func (r *kubernetesCloudResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes_cloud", "read")
		return
	}

	var state kubernetesCloudResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read the kubernetes readKubernetesCloudOutput.
	readKubernetesCloudOutput, err := r.client.Clouds.ReadKubernetesCloud(
		juju.ReadKubernetesCloudInput{
			Name: state.CloudName.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read kubernetes readKubernetesCloudOutput, got error %s", err))
		return
	}

	state.ParentCloudName = types.StringValue(readKubernetesCloudOutput.ParentCloudName)
	state.ParentCloudRegion = types.StringValue(readKubernetesCloudOutput.ParentCloudRegion)
	state.CloudName = types.StringValue(readKubernetesCloudOutput.Name)
	state.CloudCredential = types.StringValue(readKubernetesCloudOutput.CredentialName)
	state.ID = types.StringValue(newKubernetesCloudID(readKubernetesCloudOutput.Name, readKubernetesCloudOutput.CredentialName))

	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the kubernetes cloud on the controller used by Terraform provider.
func (r *kubernetesCloudResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes_cloud", "update")
		return
	}

	var plan kubernetesCloudResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the kubernetes cloud.
	err := r.client.Clouds.UpdateKubernetesCloud(
		juju.UpdateKubernetesCloudInput{
			Name:             plan.CloudName.ValueString(),
			KubernetesConfig: plan.KubernetesConfig.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update kubernetes cloud, got error %s", err))
		return
	}

	r.trace(fmt.Sprintf("Updated kubernetes cloud %s", plan.CloudName.ValueString()))
}

// Delete removes the kubernetes cloud from the controller used by Terraform provider.
func (r *kubernetesCloudResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes_cloud", "delete")
		return
	}

	var plan kubernetesCloudResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Remove the kubernetes cloud.
	err := r.client.Clouds.RemoveKubernetesCloud(
		juju.DestroyKubernetesCloudInput{
			Name: plan.CloudName.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove kubernetes cloud, got error %s", err))
		return
	}

	r.trace(fmt.Sprintf("Removed kubernetes cloud %s", plan.CloudName.ValueString()))
}

func (r *kubernetesCloudResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if r.subCtx == nil {
		return
	}
	tflog.SubsystemTrace(r.subCtx, LogResourceKubernetesCloud, msg, additionalFields...)
}

func newKubernetesCloudID(kubernetesCloudName string, cloudCredentialName string) string {
	return fmt.Sprintf("%s:%s", kubernetesCloudName, cloudCredentialName)
}
