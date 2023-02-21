package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceSSHKey() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Resource representing an SSH key.",

		CreateContext: sshKeyCreate,
		ReadContext:   sshKeyRead,
		UpdateContext: sshKeyUpdate,
		DeleteContext: sshKeyDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"payload": {
				Description: "SSH key payload.",
				Required:    true,
				ForceNew:    true,
				Sensitive:   true,
			},
		},
	}
}

func sshKeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	payload := d.Get("payload").(string)

	user := getUserFromSSHKey(payload)
	if user == "" {
		return diag.Errorf("malformed SSH key, user not found")
	}

	err = client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		Payload:   payload,
	})

	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	d.SetId(fmt.Sprintf("sshkeys:%s:%s", modelName, user))

	return diags
}

func sshKeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	user := getUserFromSSHKey(d.Get("payload").(string))
	if user == "" {
		return diag.Errorf("malformed SSH key, user not found")
	}

	result, err := client.SSHKeys.ReadSSHKey(&juju.ReadSSHKeyInput{
		ModelName: modelUUID.Name,
		ModelUUID: modelUUID.UUID,
		User:      user,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("model", result.ModelName); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("payload", result.Payload); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("sshkeys:%s:%s", modelName, user))

	return diag.Diagnostics{}
}

func sshKeyUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*juju.Client)

	if !d.HasChange("payload") {
		return diags
	}

	// any change in the payload has to be considered as a new key
	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	user := getUserFromSSHKey(d.Get("payload").(string))
	if user == "" {
		return diag.Errorf("malformed SSH key, user not found")
	}

	// delete
	err = client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		User:      user,
	})
	if err != nil {
		diags = diag.FromErr(err)
		return diags
	}

	// create again
	err = client.SSHKeys.CreateSSHKey(&juju.CreateSSHKeyInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		Payload:   d.Get("payload").(string),
	})
	if err != nil {
		diags = diag.FromErr(err)
		return diags
	}

	return diags
}

func sshKeyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	user := getUserFromSSHKey(d.Get("payload").(string))
	if user == "" {
		return diag.Errorf("malformed SSH key, user not found")
	}

	err = client.SSHKeys.DeleteSSHKey(&juju.DeleteSSHKeyInput{
		ModelName: modelUUID.Name,
		ModelUUID: modelUUID.UUID,
		User:      user,
	})

	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	d.SetId("")

	return diags
}

// getUserFromSSHKey returns the user of the key
// returning the string after the = symbol
func getUserFromSSHKey(key string) string {
	end := strings.LastIndex(key, "=")
	if end < 0 {
		return ""
	}
	return key[end+2:]
}
