package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func NewAccessModelResource() resource.Resource {
	return &accessModelResource{}
}

type accessModelResource struct {
	client *juju.Client
}

type accessModelResourceModel struct {
	Model  types.String `tfsdk:"model"`
	Users  types.List   `tfsdk:"users"`
	Access types.String `tfsdk:"access"`

	// ID required by the testing framework
	ID types.String `tfsdk:"id"`
}

func (a accessModelResource) Metadata(ctx context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	//TODO implement me
	panic("implement me")
}

func (a accessModelResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "A resource that represent a Juju Access Model.",
		Attributes: map[string]schema.Attribute{
			"model": schema.StringAttribute{
				Description: "The name of the model for access management",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"users": schema.ListAttribute{
				Description: "List of users to grant access to",
				Required:    true,
				ElementType: types.StringType,
			},
			"access": schema.StringAttribute{
				Description: "Type of access to the model",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("admin", "read", "write"),
				},
			},
		},
	}
}

func (a accessModelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan accessModelResourceModel

	// Read Terraform configuration from the request into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users basetypes.ListValue
	usersList := plan.Users.Elements()
	users := make([]string, len(usersList))
	for i, v := range usersList {
		users[i] = v.String()
	}

	modelNameStr := plan.Model.String()
	// Get the modelUUID to call Models.GrantModel
	uuid, err := a.client.Models.ResolveModelUUID(modelNameStr)
	if err != nil {
		resp.Diagnostics.AddError("ClientError", err.Error())
		return
	}
	modelUUIDs := []string{uuid}

	accessStr := plan.Access.String()
	// Call Models.GrantModel
	for _, user := range users {
		err := a.client.Models.GrantModel(juju.GrantModelInput{
			User:       user,
			Access:     accessStr,
			ModelUUIDs: modelUUIDs,
		})
		if err != nil {
			resp.Diagnostics.AddError("ClientError", err.Error())
			return
		}
	}
	plan.ID = types.StringValue(fmt.Sprintf("%s:%s", modelNameStr, accessStr))

	// Set the plan onto the Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (a accessModelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resID := strings.Split(plan.ID.String(), ":")

	// Get the users basetypes.ListValue
	usersList := plan.Users.Elements()
	stateUsers := make([]string, len(usersList))
	for i, v := range usersList {
		stateUsers[i] = v.String()
	}

	uuid, err := a.client.Models.ResolveModelUUID(resID[0])
	if err != nil {
		resp.Diagnostics.AddError("ClientError", err.Error())
		return
	}
	response, err := a.client.Users.ModelUserInfo(uuid)
	if err != nil {
		resp.Diagnostics.AddError("ClientError", err.Error())
		return
	}

	plan.Model = types.StringValue(resID[0])
	plan.Access = types.StringValue(resID[1])

	var users []string

	for _, user := range stateUsers {
		for _, modelUser := range response.ModelUserInfo {
			if user == modelUser.UserName && string(modelUser.Access) == resID[1] {
				users = append(users, modelUser.UserName)
			}
		}
	}

	uss, errDiag := basetypes.NewListValueFrom(ctx, types.StringType, users)
	plan.Users = uss
	resp.Diagnostics.Append(errDiag...)
}

func (a accessModelResource) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	//TODO implement me
	panic("implement me")
}

func (a accessModelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan accessModelResourceModel

	// Get the Terraform state from the request into the plan
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the users basetypes.ListValue
	usersList := plan.Users.Elements()
	stateUsers := make([]string, len(usersList))
	for i, v := range usersList {
		stateUsers[i] = v.String()
	}

	err := a.client.Models.DestroyAccessModel(juju.DestroyAccessModelInput{
		Model:  plan.ID.String(),
		Revoke: stateUsers,
		Access: plan.Access.String(),
	})
	if err != nil {
		resp.Diagnostics.AddError("ClientError", err.Error())
	}
}

/*
func resourceAccessModel() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represent a Juju Access Model.",

		CreateContext: resourceAccessModelCreate,
		ReadContext:   resourceAccessModelRead,
		UpdateContext: resourceAccessModelUpdate,
		DeleteContext: resourceAccessModelDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceAccessModelImporter,
		},

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model for access management",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"users": {
				Description: "List of users to grant access to",
				Type:        schema.TypeList,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"access": {
				Description:  "Type of access to the model",
				ValidateFunc: validation.StringInSlice([]string{"admin", "read", "write"}, false),
				Type:         schema.TypeString,
				ForceNew:     true,
				Required:     true,
			},
		},
	}
}

func resourceAccessModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	usersInterface := d.Get("users").([]interface{})
	stateUsers := make([]string, len(usersInterface))
	for i, v := range usersInterface {
		stateUsers[i] = v.(string)
	}

	uuid, err := client.Models.ResolveModelUUID(id[0])
	if err != nil {
		return diag.FromErr(err)
	}
	response, err := client.Users.ModelUserInfo(uuid)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("model", id[0]); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("access", id[1]); err != nil {
		return diag.FromErr(err)
	}

	var users []string

	for _, user := range stateUsers {
		for _, modelUser := range response.ModelUserInfo {
			if user == modelUser.UserName && string(modelUser.Access) == id[1] {
				users = append(users, modelUser.UserName)
			}
		}
	}

	if err = d.Set("users", users); err != nil {
		return diag.FromErr(err)
	}

	return diags
}
*/

// Updating the access model supports three cases
// access and users both changed:
// for missing users - revoke access
// for changed users - apply new access
// users changed:
// for missing users - revoke access
// for new users - apply access
// access changed - apply new access
func resourceAccessModelUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics
	anyChange := false

	// items that could be changed
	var newAccess string
	var newUsersList []string
	var missingUserList []string
	var addedUserList []string

	var err error

	if d.HasChange("users") {
		anyChange = true
		oldUsers, newUsers := d.GetChange("users")
		oldUsersInterface := oldUsers.([]interface{})
		oldUsersList := make([]string, len(oldUsersInterface))
		for i, v := range oldUsersInterface {
			oldUsersList[i] = v.(string)
		}
		newUsersInterface := newUsers.([]interface{})
		newUsersList = make([]string, len(newUsersInterface))
		for i, v := range newUsersInterface {
			newUsersList[i] = v.(string)
		}
		missingUserList = getMissingUsers(oldUsersList, newUsersList)
		addedUserList = getAddedUsers(oldUsersList, newUsersList)
	}

	if !anyChange {
		return diags
	}

	err = client.Models.UpdateAccessModel(juju.UpdateAccessModelInput{
		Model:  d.Id(),
		Grant:  addedUserList,
		Revoke: missingUserList,
		Access: newAccess,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func getMissingUsers(oldUsers, newUsers []string) []string {
	var missing []string
	for _, user := range oldUsers {
		found := false
		for _, newUser := range newUsers {
			if user == newUser {
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, user)
		}
	}
	return missing
}

func getAddedUsers(oldUsers, newUsers []string) []string {
	var added []string
	for _, user := range newUsers {
		found := false
		for _, oldUser := range oldUsers {
			if user == oldUser {
				found = true
				break
			}
		}
		if !found {
			added = append(added, user)
		}
	}
	return added
}

func resourceAccessModelImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	id := strings.Split(d.Id(), ":")
	model := id[0]
	access := id[1]
	users := strings.Split(id[2], ",")

	if err := d.Set("model", model); err != nil {
		return nil, err
	}
	if err := d.Set("access", access); err != nil {
		return nil, err
	}
	if err := d.Set("users", users); err != nil {
		return nil, err
	}

	d.SetId(fmt.Sprintf("%s:%s", model, access))

	return []*schema.ResourceData{d}, nil
}
