package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// The User resource maps to a juju user that is operated via
// `juju add-user`, `juju remove-user`
// Display name is optional.
func resourceUser() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represent a Juju User.",

		CreateContext: resourceUserCreate,
		ReadContext:   resourceUserRead,
		UpdateContext: resourceUserUpdate,
		DeleteContext: resourceUserDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "The name to be assigned to the user",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"display_name": {
				Description: "The display name to be assigned to the user",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"password": {
				Description: "The password to be assigned to the user",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func resourceUserCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	name := d.Get("name").(string)
	displayName := d.Get("display_name").(string)
	password := d.Get("password").(string)

	_, err := client.Users.CreateUser(juju.CreateUserInput{
		Name:        name,
		DisplayName: displayName,
		Password:    password,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("user:%s", name))

	return diags
}

func resourceUserRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	name := id[1]
	response, err := client.Users.ReadUser(name)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", response.UserInfo.Username); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("display_name", response.UserInfo.DisplayName); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceUserUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics
	anyChange := false

	var newPassword string

	var err error

	if d.HasChange("password") {
		anyChange = true
		newPassword = d.Get("password").(string)
	}

	if !anyChange {
		return diags
	}

	id := strings.Split(d.Id(), ":")
	name := id[1]
	err = client.Users.UpdateUser(juju.UpdateUserInput{
		Name:     name,
		Password: newPassword,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// Juju refers to user deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func resourceUserDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	name := id[1]

	err := client.Users.DestroyUser(juju.DestroyUserInput{
		Name: name,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
