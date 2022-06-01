package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceCharm() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represents a Juju Charm.",

		CreateContext: resourceCharmCreate,
		ReadContext:   resourceCharmRead,
		UpdateContext: resourceCharmUpdate,
		DeleteContext: resourceCharmDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The identifier of the model where this Charm is to be installed.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"charm": {
				Description: "The fully qualified name of the Charm to be installed.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"units": {
				Description: "The number of instances that represent the Charm.",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
			},
			"revision": {
				Description: "An the integer which is incremented each time the charm is updated in charmhub",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     -1,
			},
			"application_name": {
				Description: "A custom name for the charm. If empty, then use the charm's name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"track": {
				Description: "Supported releases of the application. Default: latest.",
				Type:        schema.TypeString,
				Default:     "latest",
				Optional:    true,
			},
			"risk": {
				Description: "Type of stability of the charm. Default: stable.",
				Type:        schema.TypeString,
				Default:     "stable",
				Optional:    true,
			},
			"branch": {
				Description: "A branch with temporary releases of the charm.",
				Type:        schema.TypeString,
				Optional:    true,
			},
		},
	}
}

func resourceCharmCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceCharmRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceCharmUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceCharmDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}
