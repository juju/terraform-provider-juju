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
		},
	}
}

func dataSourceModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("name").(string)

	model, err := client.Models.GetByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(model.UUID)
	d.Set("uuid", model.UUID)
	d.Set("name", model.Name)

	return nil
}
