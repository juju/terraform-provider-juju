package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func dataSourceModel() *schema.Resource {
	return &schema.Resource{
		Description: "A data source representing a Juju Model.",
		ReadContext: dataSourceModelRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Description: "The name of the model.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"uuid": {
				Description: "The UUID of the model.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func dataSourceModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("name").(string)

	model, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(model.UUID)
	if err = d.Set("uuid", model.UUID); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", model.Name); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
