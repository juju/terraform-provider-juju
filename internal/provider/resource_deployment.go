package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceDeployment() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju deployment.",

		CreateContext: resourceDeploymentCreate,
		ReadContext:   resourceDeploymentRead,
		UpdateContext: resourceDeploymentUpdate,
		DeleteContext: resourceDeploymentDelete,

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A custom name for the application deployment. If empty, uses the charm's name.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"model": {
				Description: "The name of the model where the charm is to be deployed.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"charm": {
				Description: "The name of the charm to be installed from Charmhub.",
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
						"channel": {
							Description: "The channel to use when deploying a charm. Specified as <track>/<risk>/<branch>.",
							Type:        schema.TypeString,
							Default:     "latest/stable",
							Optional:    true,
						},
						"revision": {
							Description: "The revision of the charm to deploy.",
							Type:        schema.TypeInt,
							Default:     juju.UnspecifiedRevision,
							Optional:    true,
						},
						"series": {
							Description: "The series on which to deploy.",
							Type:        schema.TypeString,
							Default:     "",
							Optional:    true,
						},
					},
				},
			},
			"units": {
				Description: "The number of application units to deploy for the charm.",
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
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
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	name := d.Get("name").(string)
	charm := d.Get("charm").([]interface{})[0].(map[string]interface{})
	charmName := charm["name"].(string)
	channel := charm["channel"].(string)
	revision := charm["revision"].(int)
	series := charm["series"].(string)
	units := d.Get("units").(int)

	deployedName, err := client.Deployments.CreateDeployment(&juju.CreateDeploymentInput{
		ApplicationName: name,
		ModelUUID:       modelUUID,
		CharmName:       charmName,
		CharmChannel:    channel,
		CharmRevision:   revision,
		CharmSeries:     series,
		Units:           units,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	// TODO: id generation - is there a natural ID we can use?
	d.SetId(fmt.Sprintf("%s/%s", modelUUID, deployedName))

	return nil
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
