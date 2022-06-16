package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceModel() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represent a Juju Model.",

		CreateContext: resourceModelCreate,
		ReadContext:   resourceModelRead,
		UpdateContext: resourceModelUpdate,
		DeleteContext: resourceModelDelete,

		Schema: map[string]*schema.Schema{
			// TODO: this needs to be reviewed
			"name": {
				Description: "The name to be assigned to the model",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"controller": {
				Description: "The name of the controller to target. Optional",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
			},
			"cloud": {
				Description: "JuJu Cloud where the model will operate",
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Description: "The name of the cloud",
							Type:        schema.TypeString,
							Required:    true,
						},
						"region": &schema.Schema{
							Description: "The region of the cloud",
							Type:        schema.TypeString,
							Optional:    true,
						},
					},
				},
			},
			"config": {
				Description: "Override default model configuration.",
				Type:        schema.TypeMap,
				Optional:    true,
			},
			"type": {
				Description: "Type of the model. Set by the Juju's API server",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceModelCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	client := meta.(*juju.Client)

	name := d.Get("name").(string)
	controller := d.Get("controller").(string)
	cloud := d.Get("cloud").([]interface{})
	config := d.Get("config").(map[string]interface{})

	modelInfo, err := client.Models.Create(name, controller, cloud, config)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(modelInfo.UUID)

	return nil
}

func resourceModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	client := meta.(*juju.Client)

	uuid := d.Id()
	controllerName, modelInfo, err := client.Models.Read(uuid)
	if err != nil {
		return diag.FromErr(err)
	}

	cloudList := []map[string]interface{}{{"name": modelInfo.CloudTag, "region": modelInfo.CloudRegion}}

	if err := d.Set("name", modelInfo.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("controller", controllerName); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("cloud", cloudList); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", modelInfo.Type); err != nil {
		return diag.FromErr(err)
	}
	// TODO: locate model config values form modelsAPI or other endpoint.

	return nil
}

func resourceModelUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceModelDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelUUID := d.Id()

	err := client.Models.Delete(modelUUID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
