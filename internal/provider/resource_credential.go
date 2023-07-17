package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
		Attributes: map[string]schema.Attribute{
			"cloud": schema.ListNestedAttribute{
				Description: "JuJu Cloud where the credentials will be used to access",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Description: "The name of the cloud",
							Required:    true,
						},
					},
				},
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
			},
			"attributes": schema.MapAttribute{
				Description: "Credential attributes accordingly to the cloud",
				ElementType: types.StringType,
				Optional:    true,
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
		},
	}
}

func (c *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the resource fields
	var attributesRaw map[string]interface{}
	var authType string
	var clientCredential bool
	var cloud []interface{}
	var controllerCredential bool
	var credentialName string

	plan.Attributes.ElementsAs(ctx, attributesRaw, false)
	authType = plan.AuthType.ValueString()
	clientCredential = plan.ClientCredential.ValueBool()
	plan.Cloud.ElementsAs(ctx, cloud, false)
	controllerCredential = plan.ControllerCredential.ValueBool()
	credentialName = plan.Name.ValueString()

	attributes := make(map[string]string)
	for key, value := range attributesRaw {
		attributes[key] = AttributeEntryToString(value)
	}
	// Prevent a segfault if client is not yet configured
	if c.client == nil {
		resp.Diagnostics.AddError(
			"Credential Resource - Create : Client Not Configured",
			"Expected configured Juju Client. Please report this issue to the provider developers.",
		)
		return
	}
	response, err := c.client.Credentials.CreateCredential(juju.CreateCredentialInput{
		Attributes:           attributes,
		AuthType:             authType,
		ClientCredential:     clientCredential,
		CloudList:            cloud,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s:%t:%t", credentialName, response.CloudName, clientCredential, controllerCredential))

	// Set the desired plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func AttributeEntryToString(input interface{}) string {
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

func (c *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve and validate the ID
	resID := strings.Split(plan.ID.ValueString(), ":")
	if len(resID) != 4 {
		resp.Diagnostics.AddError("Provider Error - Credential Resource : Read",
			fmt.Sprintf("Invalid ID - expected {credentialName, cloudName, isClient, isController} - given : %v",
				resID))
		return
	}
	// Extract fields from the ID
	credentialName, cloudName, clientCredentialStr, controllerCredentialStr := resID[0], resID[1], resID[2], resID[3]

	cloudList := []map[string]interface{}{{
		"name": cloudName,
	}}

	cloud, errDiag := basetypes.NewListValueFrom(ctx, types.ObjectType{}, cloudList)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Cloud = cloud

	clientCredential, controllerCredential, err := convertOptionsBool(clientCredentialStr, controllerCredentialStr)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	plan.ClientCredential = types.BoolValue(clientCredential)
	plan.ControllerCredential = types.BoolValue(controllerCredential)

	// Prevent runtime to freak out if client is not configured
	if c.client == nil {
		resp.Diagnostics.AddError(
			"Credential Resource - Read : Client Not Configured",
			"Expected configured Juju Client. Please report this issue to the provider developers.",
		)
		return
	}
	response, err := c.client.Credentials.ReadCredential(juju.ReadCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	plan.Name = types.StringValue(response.CloudCredential.Label)
	plan.AuthType = types.StringValue(string(response.CloudCredential.AuthType()))

	receivedAttributes := response.CloudCredential.Attributes()

	var configuredAttributes map[string]interface{}
	plan.Attributes.ElementsAs(ctx, configuredAttributes, false)
	for configAtr := range configuredAttributes {
		if receivedValue, exists := receivedAttributes[configAtr]; exists {
			configuredAttributes[configAtr] = AttributeEntryToString(receivedValue)
		}
	}

	plan.Attributes, errDiag = basetypes.NewMapValueFrom(ctx, types.StringType, configuredAttributes)
	resp.Diagnostics.Append(errDiag...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
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

func (c *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Return early if no change
	if plan.AuthType.Equal(state.AuthType) &&
		plan.ClientCredential.Equal(state.ClientCredential) &&
		plan.ControllerCredential.Equal(state.ControllerCredential) &&
		plan.Attributes.Equal(state.Attributes) {
		return
	}

	// Retrieve and validate the ID
	resID := strings.Split(plan.ID.ValueString(), ":")
	if len(resID) != 4 {
		resp.Diagnostics.AddError("Provider Error - Credential Resource : Read",
			fmt.Sprintf("Invalid ID - expected {credentialName, cloudName, isClient, isController} - given : %v",
				resID))
		return
	}
	// Extract fields from the ID for the UpdateCredentialInput call
	credentialName, cloudName := resID[0], resID[1]

	newAuthType := plan.AuthType.ValueString()
	newClientCredential := plan.ClientCredential.ValueBool()
	newControllerCredential := plan.ControllerCredential.ValueBool()
	var attributesRaw map[string]interface{}
	plan.Attributes.ElementsAs(ctx, attributesRaw, false)
	newAttributes := make(map[string]string)
	for key, value := range attributesRaw {
		newAttributes[key] = AttributeEntryToString(value)
	}

	// Prevent runtime to freak out if client is not configured
	if c.client == nil {
		resp.Diagnostics.AddError(
			"Credential Resource - Update : Client Not Configured",
			"Expected configured Juju Client. Please report this issue to the provider developers.",
		)
		return
	}

	err := c.client.Credentials.UpdateCredential(juju.UpdateCredentialInput{
		Attributes:           newAttributes,
		AuthType:             newAuthType,
		ClientCredential:     newClientCredential,
		CloudName:            cloudName,
		ControllerCredential: newControllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
		return
	}

	newID := fmt.Sprintf("%s:%s:%t:%t", credentialName, cloudName, newClientCredential, newControllerCredential)
	plan.ID = types.StringValue(newID)

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (c *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan credentialResourceModel

	// Read Terraform configuration from the request into the resource model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Retrieve and validate the ID
	resID := strings.Split(plan.ID.ValueString(), ":")
	if len(resID) != 4 {
		resp.Diagnostics.AddError("Provider Error - Credential Resource : Read",
			fmt.Sprintf("Invalid ID - expected {credentialName, cloudName, isClient, isController} - given : %v",
				resID))
		return
	}
	// Extract fields from the ID
	credentialName, cloudName, clientCredentialStr, controllerCredentialStr := resID[0], resID[1], resID[2], resID[3]
	clientCredential, controllerCredential, err := convertOptionsBool(clientCredentialStr, controllerCredentialStr)
	if err != nil {
		resp.Diagnostics.AddError("Provider Error", err.Error())
		return
	}
	// Prevent runtime to freak out if client is not configured
	if c.client == nil {
		resp.Diagnostics.AddError(
			"Credential Resource - Read : Client Not Configured",
			"Expected configured Juju Client. Please report this issue to the provider developers.",
		)
		return
	}
	err = c.client.Credentials.DestroyCredential(juju.DestroyCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", err.Error())
	}
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
}

func (c credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
