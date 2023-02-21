package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

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

func resourceAccessModelCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	model := d.Get("model").(string)
	access := d.Get("access").(string)
	usersInterface := d.Get("users").([]interface{})
	users := make([]string, len(usersInterface))
	for i, v := range usersInterface {
		users[i] = v.(string)
	}

	uuid, err := client.Models.ResolveModelUUID(model)
	if err != nil {
		return diag.FromErr(err)
	}

	modelUUIDs := []string{uuid}

	for _, user := range users {
		err := client.Models.GrantModel(juju.GrantModelInput{
			User:       user,
			Access:     access,
			ModelUUIDs: modelUUIDs,
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(fmt.Sprintf("%s:%s", model, access))

	return diags
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

// resourceAccessModelDelete deletes the access model resource
// Juju refers to deletions as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func resourceAccessModelDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	usersInterface := d.Get("users").([]interface{})
	users := make([]string, len(usersInterface))
	for i, v := range usersInterface {
		users[i] = v.(string)
	}
	access := d.Get("access").(string)

	err := client.Models.DestroyAccessModel(juju.DestroyAccessModelInput{
		Model:  d.Id(),
		Revoke: users,
		Access: access,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
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
