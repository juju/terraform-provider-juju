package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDeployment() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju deployment.",

		CreateContext: resourceDeploymentCreate,
		ReadContext:   resourceDeploymentRead,
		UpdateContext: resourceDeploymentUpdate,
		DeleteContext: resourceDeploymentDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The identifier of the model where this Charm is to be installed.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"charm": {
				Description: "The name of the Charm to be installed from Charmhub.",
				Type:        schema.TypeList,
				Required:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Description: "The name of the charm",
							Type:        schema.TypeString,
							Required:    true,
						},
						"revision": &schema.Schema{
							Description: "The revision of the charm to deploy.",
							Type:        schema.TypeInt,
							Optional:    true,
							Default:     -1,
						},
						"channel": {
							Description: "The channel to use when deploying a charm.",
							Type:        schema.TypeString,
							Default:     "stable",
							Optional:    true,
						},
					},
				},
			},
			"units": {
				Description: "The number of instances that represent the Charm.",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
			},
			"name": {
				Description: "A custom name for the application deployment. If empty, uses the charm's name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"config": {
				Description: "Application specific configuration.",
				Type:        schema.TypeMap,
				Optional:    true,
			},
		},
	}
}

func resourceDeploymentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceDeploymentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceDeploymentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceDeploymentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}
