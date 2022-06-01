package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceModel() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A data source representing a Juju Model.",

		ReadContext: dataSourceModelRead,

		Schema: map[string]*schema.Schema{
			"name": {
				// This description is used by the documentation generator and the language server.
				Description: "The name to be assigned to the model.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func dataSourceModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// use the meta value to retrieve your client from the provider configure method
	// client := meta.(*apiClient)

	idFromAPI := "my-id"
	d.SetId(idFromAPI)

	return diag.Errorf("not implemented")
}
