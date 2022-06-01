package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRelation() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represents a Juju Relation.",

		CreateContext: resourceRelationCreate,
		ReadContext:   resourceRelationRead,
		UpdateContext: resourceRelationUpdate,
		DeleteContext: resourceRelationDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "Model",
				Type:        schema.TypeString,
				Required:    true,
			},
			"src": {
				Description: "The name of an application providing the relation.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"dst": {
				Description: "The name of an application requiring the relation",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"relations": {
				Description: "The name of the relation as known by both charms.",
				Type:        schema.TypeList,
				Required:    true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func resourceRelationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceRelationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceRelationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceRelationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}
