// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &credentialResource{}
var _ resource.ResourceWithConfigure = &credentialResource{}
var _ resource.ResourceWithImportState = &credentialResource{}

func NewCredentialResource() resource.Resource {
	return &credentialResource{}
}

type credentialResource struct {
	client *juju.Client

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

type credentialResourceModel struct {
	Cloud                types.List   `tfsdk:"cloud"`
	Attributes           types.Map    `tfsdk:"attributes"`
	AuthType             types.String `tfsdk:"auth_type"`
	ClientCredential     types.Bool   `tfsdk:"client_credential"`
	ControllerCredential types.Bool   `tfsdk:"controller_credential"`
	Name                 types.String `tfsdk:"name"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (c *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (c *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a credential for a cloud.",
		Blocks: map[string]schema.Block{
			"cloud": schema.ListNestedBlock{
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the cloud",
							Required:    true,
						},
					},
				},
				Description: "JuJu Cloud where the credentials will be used to access",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
		},
		Attributes: map[string]schema.Attribute{
			"attributes": schema.MapAttribute{
				Description: "Credential attributes accordingly to the cloud",
				ElementType: types.StringType,
				Optional:    true,
				Sensitive:   true,
			},
			"auth_type": schema.StringAttribute{
				Description: "Credential authorization type",
				Required:    true,
			},
			"client_credential": schema.BoolAttribute{
				Description: "Add credentials to the client",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
			},
			"controller_credential": schema.BoolAttribute{
				Description: "Add credentials to the controller",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
			},
			"name": schema.StringAttribute{
				Description: "The name to be assigned to the credential",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			// ID required by the testing framework
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (c *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Check first if the client is configured
	if c.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "credential", "create")
		return
	}

	var data credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Access the fields
	// attributes
	var attributes map[string]string
	resp.Diagnostics.Append(data.Attributes.ElementsAs(ctx, &attributes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// auth_type
	authType := data.AuthType.ValueString()

	// cloud.name
	cloudName, errDiag := cloudNameFromCredentialCloud(ctx, data.Cloud.Elements()[0], resp.Diagnostics)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}

	// client_credential
	clientCredential := data.ClientCredential.ValueBool()

	// controller_credential
	controllerCredential := data.ControllerCredential.ValueBool()

	// name
	credentialName := data.Name.ValueString()

	// Perform logic or external calls
	response, err := c.client.Credentials.CreateCredential(juju.CreateCredentialInput{
		Attributes:           attributes,
		AuthType:             authType,
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create credential resource, got error: %s", err))
		return
	}
	c.trace(fmt.Sprintf("created credential resource %q", credentialName))

	data.ID = types.StringValue(newCredentialIDFrom(credentialName, response.CloudName, clientCredential, controllerCredential))

	// Write the state data into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	// Check first if the client is configured
	if c.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "credential", "read")
		return
	}

	var data credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Access prior state data

	credentialName, cloudName, clientCredential, controllerCredential := retrieveCredentialDataFromID(data.ID.ValueString(), &resp.Diagnostics,
		"read")
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve updated resource state from upstream
	response, err := c.client.Credentials.ReadCredential(juju.ReadCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		// TODO (cderici): call resp.State.RemoveResource() if NotFound
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read credential resource, got error: %s", err))
		return
	}
	c.trace(fmt.Sprintf("read credential resource %q", credentialName))

	// cloud
	cloud, errDiag := newCredentialCloudFromCloudName(ctx, cloudName, resp.Diagnostics)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Cloud = cloud

	// client_credential & controller_credential
	data.ClientCredential = types.BoolValue(clientCredential)
	data.ControllerCredential = types.BoolValue(controllerCredential)

	// retrieve name & auth_type
	data.Name = types.StringValue(response.CloudCredential.Label)
	data.AuthType = types.StringValue(string(response.CloudCredential.AuthType()))

	// retrieve the attributes
	receivedAttributes := response.CloudCredential.Attributes()
	if len(receivedAttributes) > 0 {
		var configuredAttributes map[string]string
		resp.Diagnostics.Append(data.Attributes.ElementsAs(ctx, &configuredAttributes, false)...)
		if resp.Diagnostics.HasError() {
			return
		}

		for configAtr := range configuredAttributes {
			if receivedValue, exists := receivedAttributes[configAtr]; exists {
				configuredAttributes[configAtr] = attributeEntryToString(receivedValue)
			}
		}

		if len(configuredAttributes) != 0 {
			data.Attributes, errDiag = types.MapValueFrom(ctx, types.StringType, configuredAttributes)
			resp.Diagnostics.Append(errDiag...)
			if resp.Diagnostics.HasError() {
				return
			}
		}
	}

	// Write the state data into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Check first if the client is configured
	if c.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "credential", "update")
		return
	}

	var data, state credentialResourceModel

	// Read current state of resource prior to the update into the 'state' model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read desired state of resource after the update into the 'data' model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Return early if no change
	// No need to check the name and cloud.name because they can't be updated in-place without recreating the resource
	// i.e. their change will force recreation of the resource (see the schema)
	if data.AuthType.Equal(state.AuthType) &&
		data.ClientCredential.Equal(state.ClientCredential) &&
		data.ControllerCredential.Equal(state.ControllerCredential) &&
		data.Attributes.Equal(state.Attributes) {
		return
	}

	// Extract fields from the ID for the UpdateCredentialInput call
	// name & cloud.name fields
	credentialName, cloudName, _, _ := retrieveCredentialDataFromID(data.ID.ValueString(), &resp.Diagnostics, "update")
	if resp.Diagnostics.HasError() {
		return
	}

	// auth_type
	newAuthType := data.AuthType.ValueString()

	// client_credential & controller_credential
	newClientCredential := data.ClientCredential.ValueBool()
	newControllerCredential := data.ControllerCredential.ValueBool()

	// attributes
	var newAttributes map[string]string
	resp.Diagnostics.Append(data.Attributes.ElementsAs(ctx, &newAttributes, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Perform external call to modify resource
	err := c.client.Credentials.UpdateCredential(juju.UpdateCredentialInput{
		Attributes:           newAttributes,
		AuthType:             newAuthType,
		ClientCredential:     newClientCredential,
		CloudName:            cloudName,
		ControllerCredential: newControllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update credential resource, got error: %s", err))
		return
	}
	c.trace(fmt.Sprintf("updated credential resource %q", credentialName))

	data.ID = types.StringValue(newCredentialIDFrom(credentialName, cloudName, newClientCredential, newControllerCredential))

	// Write the updated state data into the Response.State
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (c *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	// Check first if the client is configured
	if c.client == nil {
		addClientNotConfiguredError(&resp.Diagnostics, "credential", "delete")
		return
	}

	var data credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Access prior state data

	// extract : name & cloud.name, client_credential, controller_credential
	credentialName, cloudName, clientCredential, controllerCredential := retrieveCredentialDataFromID(data.ID.ValueString(), &resp.Diagnostics,
		"update")
	if resp.Diagnostics.HasError() {
		return
	}

	// Perform external call to destroy the resource
	err := c.client.Credentials.DestroyCredential(juju.DestroyCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete credential resource, got error: %s", err))
	}
	c.trace(fmt.Sprintf("deleted credential resource %q", credentialName))
}

func (c *credentialResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	c.client = client
	// Create the local logging subsystem here, using the TF context when creating it.
	c.subCtx = tflog.NewSubsystem(ctx, LogResourceCredential)
}

func (c credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (c *credentialResource) trace(msg string, additionalFields ...map[string]interface{}) {
	if c.subCtx == nil {
		return
	}

	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemTrace(c.subCtx, LogResourceCredential, msg, additionalFields...)
}

func cloudNameFromCredentialCloud(ctx context.Context, element attr.Value, diag diag.Diagnostics) (string,
	diag.Diagnostics) {
	blockAttributeType := map[string]attr.Type{
		"name": types.StringType,
	}
	cloudObj, errDiag := types.ObjectValueFrom(ctx, blockAttributeType, element)
	diag.Append(errDiag...)
	if diag.HasError() {
		return "", diag
	}
	return cloudObj.Attributes()["name"].(types.String).ValueString(), diag
}

func newCredentialCloudFromCloudName(ctx context.Context, cloudName string, diag diag.Diagnostics) (types.List, diag.Diagnostics) {
	cloudAttributes := map[string]attr.Value{
		"name": types.StringValue(cloudName),
	}
	blockAttributeType := map[string]attr.Type{
		"name": types.StringType,
	}

	cloudBlock, errDiag := types.ObjectValue(blockAttributeType, cloudAttributes)
	diag.Append(errDiag...)
	if diag.HasError() {
		return types.ListNull(types.StringType), diag
	}

	attrType := types.ObjectType{AttrTypes: blockAttributeType}
	cloud, errDiag := types.ListValueFrom(ctx, attrType, []attr.Value{cloudBlock})
	diag.Append(errDiag...)
	if diag.HasError() {
		return types.ListNull(types.StringType), diag
	}
	return cloud, diag
}

func newCredentialIDFrom(credentialName string, cloudName string, clientCredential bool, controllerCredential bool) string {
	return fmt.Sprintf("%s:%s:%t:%t", credentialName, cloudName, clientCredential, controllerCredential)
}

func retrieveCredentialDataFromID(idStr string, diag *diag.Diagnostics, method string) (string, string, bool, bool) {
	resID := strings.Split(idStr, ":")
	if len(resID) != 4 {
		diag.AddError("Provider Error",
			fmt.Sprintf("unable to %s credential resource, invalid ID, expected {credentialName, cloudName, "+
				"isClient, isController} - given : %q",
				method, resID))
		return "", "", false, false
	}
	credentialName, cloudName, clientCredentialStr, controllerCredentialStr := resID[0], resID[1], resID[2], resID[3]
	clientCredential, controllerCredential, err := convertOptionsBool(clientCredentialStr, controllerCredentialStr)
	if err != nil {
		diag.AddError("Provider Error",
			fmt.Sprintf("Unable to %s credential resource, got error: %s", method, err))
		return "", "", false, false
	}
	return credentialName, cloudName, clientCredential, controllerCredential
}

func attributeEntryToString(input interface{}) string {
	switch t := input.(type) {
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', 0, 64)
	default:
		return input.(string)
	}
}

func convertOptionsBool(clientCredentialStr, controllerCredentialStr string) (bool, bool, error) {
	clientCredentialBool, err := strconv.ParseBool(clientCredentialStr)
	if err != nil {
		return false, false, fmt.Errorf("unable to parse client credential from provided ID")
	}

	controllerCredentialBool, err := strconv.ParseBool(controllerCredentialStr)
	if err != nil {
		return false, false, fmt.Errorf("unable to parse controller credential from provided ID")
	}

	return clientCredentialBool, controllerCredentialBool, nil
}
