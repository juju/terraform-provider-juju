package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceSSHKeys() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "Resource representing an SSH key.",

		CreateContext: sshKeysCreate,
		ReadContext:   sshKeysRead,
		UpdateContext: sshKeysUpdate,
		DeleteContext: sshKeysDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"key": {
				Description: "SSH key.",
				Type:        schema.TypeSet,
				Required:    true,
				MinItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"payload": {
							Description: "SSH key payload.",
							Type:        schema.TypeString,
							Required:    true,
						},
					},
				},
			},
		},
	}
}

func sshKeysCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	keyEntries := d.Get("key").(*schema.Set).List()
	keys := make([]string, len(keyEntries))
	for i, entry := range keyEntries {
		m := entry.(map[string]interface{})
		keys[i] = m["payload"].(string)
	}

	err = client.SSHKeys.CreateSSHKeys(&juju.CreateSSHKeysInput{
		ModelName: modelName,
		ModelUUID: modelUUID,
		Keys:      keys,
	})

	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	d.SetId(fmt.Sprintf("keys-%s", modelName))

	return diags
}

func sshKeysRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	result, err := client.SSHKeys.ReadSSHKeys(&juju.ReadSSHKeysInput{
		ModelName: modelUUID.Name,
		ModelUUID: modelUUID.UUID,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("model", result.ModelName); err != nil {
		return diag.FromErr(err)
	}

	// process the keys
	keys := make([]map[string]interface{}, len(result.Keys))
	for i, key := range result.Keys {
		keys[i] = map[string]interface{}{
			"payload": key,
		}
	}

	if err = d.Set("key", keys); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("keys-%s", modelName))

	return diag.Diagnostics{}
}

func sshKeysUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	foundChanges := false
	if d.HasChange("model") {
		modelName := d.Get("model").(string)
		d.Set("model", modelName)
		foundChanges = true
		d.SetId(fmt.Sprintf("keys-%s", modelName))
	}

	// Find new/removed keys
	old, new := d.GetChange("key")
	oldKeysEntry := old.(*schema.Set).List()
	newKeysEntry := new.(*schema.Set).List()

	toAdd := make([]string, 0)
	toRemove := make([]string, len(newKeysEntry))
	for i, k := range oldKeysEntry {
		payload := k.(map[string]interface{})["payload"].(string)
		toRemove[i] = payload
	}

	found := false

	for _, newEntry := range newKeysEntry {
		m := newEntry.(map[string]interface{})
		newKey := m["payload"].(string)
		found = false
		for i, oldEntry := range toRemove {
			if oldEntry == newKey {
				// we found the key. Do not add, do not remove.
				found = true
				// no need to remove the old one
				toRemove = append(toRemove[:i], toRemove[i+1:]...)
				break
			}
		}
		if !found {
			// A key was not found. Stop
			toAdd = append(toAdd, newKey)
			foundChanges = true
		}
	}

	if !foundChanges {
		// nothing changed
		return diag.Diagnostics{}
	}

	// Compare new and old keys.
	// Remove old keys not found in the new set.
	// Create new keys not found in the old set.
	client := meta.(*juju.Client)
	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	if len(toAdd) != 0 {
		err = client.SSHKeys.CreateSSHKeys(&juju.CreateSSHKeysInput{
			ModelName: modelName,
			ModelUUID: modelUUID,
			Keys:      toAdd,
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if len(toRemove) != 0 {
		err = client.SSHKeys.DeleteSSHKeys(&juju.DeleteSSHKeysInput{
			ModelName: modelName,
			ModelUUID: modelUUID,
			Keys:      toRemove,
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("key", new); err != nil {
		return diag.FromErr(err)
	}
	return diags
}

func sshKeysDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	keyEntries := d.Get("key").(*schema.Set).List()
	keys := make([]string, len(keyEntries))
	for i, entry := range keyEntries {
		m := entry.(map[string]interface{})
		keys[i] = m["payload"].(string)
	}

	err = client.SSHKeys.DeleteSSHKeys(&juju.DeleteSSHKeysInput{
		ModelName: modelUUID.Name,
		ModelUUID: modelUUID.UUID,
		Keys:      keys,
	})
	if err != nil {
		diags = append(diags, diag.FromErr(err)...)
	}

	d.SetId("")

	return diags
}
