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

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &kubernetesCloudResource{}
var _ resource.ResourceWithConfigure = &kubernetesCloudResource{}
var _ resource.ResourceWithConfigValidators = &kubernetesCloudResource{}

func NewKubernetesCloudResource() resource.Resource {
	return &kubernetesCloudResource{}
}

type kubernetesCloudResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type kubernetesCloudResourceModel struct {
	CloudName                  types.String `tfsdk:"name"`
	CloudCredential            types.String `tfsdk:"credential"`
	KubernetesConfig           types.String `tfsdk:"kubernetes_config"`
	ParentCloudName            types.String `tfsdk:"parent_cloud_name"`
	ParentCloudRegion          types.String `tfsdk:"parent_cloud_region"`
	SkipServiceAccountCreation types.Bool   `tfsdk:"skip_service_account_creation"`
	StorageClassName           types.String `tfsdk:"storage_class_name"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

const StorageClassNameMarkdownDescription = `Specify the Kubernetes storage class name for workload and operator storage.

When adding K8S clouds via the Terraform Provider, it strays in behaviour from the Juju CLI.

The Juju CLI's add-k8s command has a --storage flag which allows users to specify
a storage class name to be used for both operator and workload storage.

The Juju CLI also has a --skip-storage flag which prevents Juju from configuring any
storage class names on the cloud definition. By default, this is false.

When adding a K8S cloud via the Juju CLI, it intelligently selects storage classes
based on cloud provider preferences (e.g., 'gp2' for AWS, 'standard' for GCE) if no
storage class is specified via the --storage flag.

This intelligent selection is not implemented in the Terraform Provider as it requires
direct communication with the Kubernetes cluster in question to be added as a cloud.
That is, when running terraform and attempting to add a Kubernetes cloud, the caller
would need network connectivity to the cluster.

Instead, we expect users to explicitly define the storage class name to use for
operator and workload storage via this attribute and default to no storage class specified 
otherwise (equivalent to --skip-storage=true in the Juju CLI).

To find this information, users can query their cluster directly, e.g. via:
  kubectl get storageclass
`

// Configure is used to configure the kubernetes cloud resource.
func (r *kubernetesCloudResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	provider, diags := getProviderData(req, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.client = provider.Client
	// Create the local logging subsystem here, using the TF context when creating it.
	r.subCtx = tflog.NewSubsystem(ctx, LogResourceKubernetesCloud)
}

// ConfigValidators returns a list of functions which will all be performed during validation.
func (r *kubernetesCloudResource) ConfigValidators(context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		&kuberenetesCloudJAASValidator{r.client},
	}
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
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kubernetes_config": schema.StringAttribute{
				Description: "The kubernetes config file path for the cloud. Cloud credentials will be added to the Juju controller for you.",
				Optional:    true,
				Sensitive:   true,
			},
			"parent_cloud_name": schema.StringAttribute{
				Description: "The parent cloud name, for adding a k8s cluster from an existing cloud. Changing this value will cause the cloud to be destroyed and recreated by terraform. *Note* that this value must be set when running against a JAAS controller.",
				Optional:    true,
			},
			"parent_cloud_region": schema.StringAttribute{
				Description: "The parent cloud region name, for adding a k8s cluster from an existing cloud. Changing this value will cause the cloud to be destroyed and recreated by terraform. *Note* that this value must be set when running against a JAAS controller.",
				Optional:    true,
			},
			"skip_service_account_creation": schema.BoolAttribute{
				Description: "If set to true, the Juju Terraform provider will not create a service account and associated role within the K8s cluster and override the authentication info in the K8s config. " +
					"This way it does not need to connect to the K8s API when adding a k8s cloud.",
				Optional: true,
			},
			"storage_class_name": schema.StringAttribute{
				Description:         "Specify the Kubernetes storage class name for workload and operator storage.",
				MarkdownDescription: StorageClassNameMarkdownDescription,
				Optional:            true,
			},
			// ID is required by the testing framework.
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

	if plan.StorageClassName.ValueString() == "" {
		resp.Diagnostics.AddWarning(
			"Storage Class Name Not Set",
			"No storage class name has been set. "+
				"This may lead to issues if no storage is unsuitable for your environment. "+
				"Consider setting the storage_class_name attribute to ensure proper storage configuration.")
	}

	// Create the kubernetes cloud.
	cloudCredentialName, err := r.client.Clouds.CreateKubernetesCloud(
		&juju.CreateKubernetesCloudInput{
			Name:                 plan.CloudName.ValueString(),
			KubernetesConfig:     plan.KubernetesConfig.ValueString(),
			ParentCloudName:      plan.ParentCloudName.ValueString(),
			ParentCloudRegion:    plan.ParentCloudRegion.ValueString(),
			CreateServiceAccount: !plan.SkipServiceAccountCreation.ValueBool(),
			StorageClassName:     plan.StorageClassName.ValueString(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create kubernetes cloud, got error %s", err))
		return
	}

	plan.CloudCredential = types.StringValue(cloudCredentialName)
	plan.ID = types.StringValue(newKubernetesCloudID(plan.CloudName.ValueString(), cloudCredentialName))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)

	r.trace(fmt.Sprintf("Created kubernetes cloud %s", plan.CloudName.ValueString()))
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

	state.CloudName = types.StringValue(readKubernetesCloudOutput.Name)
	state.CloudCredential = types.StringValue(readKubernetesCloudOutput.CredentialName)
	state.ID = types.StringValue(newKubernetesCloudID(readKubernetesCloudOutput.Name, readKubernetesCloudOutput.CredentialName))

	r.trace(fmt.Sprintf("Read kubernetes cloud %s", state.CloudName))

	// Set the state onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update updates the kubernetes cloud on the controller used by Terraform provider.
func (r *kubernetesCloudResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if r.client.IsJAAS() {
		resp.Diagnostics.AddError("Not Supported", "Cloud Update is not supported in JAAS.")
		return
	}
	// Prevent panic if the provider has not been configured.
	if r.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "kubernetes_cloud", "update")
		return
	}

	var plan kubernetesCloudResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the kubernetes cloud.
	err := r.client.Clouds.UpdateKubernetesCloud(
		juju.UpdateKubernetesCloudInput{
			Name:                 plan.CloudName.ValueString(),
			KubernetesConfig:     plan.KubernetesConfig.ValueString(),
			ParentCloudName:      plan.ParentCloudName.ValueString(),
			ParentCloudRegion:    plan.ParentCloudRegion.ValueString(),
			CreateServiceAccount: !plan.SkipServiceAccountCreation.ValueBool(),
		},
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update kubernetes cloud, got error %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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
	err := r.client.Clouds.RemoveCloud(
		juju.RemoveCloudInput{
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

type kuberenetesCloudJAASValidator struct {
	client *juju.Client
}

// Description implements the Description method of the resource.ConfigValidator interface.
func (v *kuberenetesCloudJAASValidator) Description(ctx context.Context) string {
	return v.MarkdownDescription(ctx)
}

// MarkdownDescription implements the MarkdownDescription method of the resource.ConfigValidator interface.
func (v *kuberenetesCloudJAASValidator) MarkdownDescription(_ context.Context) string {
	return "Enforces that the parent_cloud_name is specified when applying to a JAAS controller."
}

// ValidateResource implements the ValidateResource method of the resource.ConfigValidator interface.
func (v *kuberenetesCloudJAASValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	if v.client == nil {
		return
	}

	if !v.client.IsJAAS() {
		return
	}

	var data kubernetesCloudResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if data.ParentCloudName.ValueString() == "" {
		resp.Diagnostics.AddError("Plan Error", "Field `parent_cloud_name` must be specified when applying to a JAAS controller.")
	}
}

func newKubernetesCloudID(kubernetesCloudName string, cloudCredentialName string) string {
	return fmt.Sprintf("%s:%s", kubernetesCloudName, cloudCredentialName)
}
