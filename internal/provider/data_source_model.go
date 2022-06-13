package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/juju/api"
	"github.com/juju/terraform-provider-juju/internal/juju/client"
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
	conn := meta.(api.Connection)

	juju := client.New(conn)

	modelName := d.Get("name").(string)

	model, err := juju.Models.GetByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(model.UUID)

	return nil
}
