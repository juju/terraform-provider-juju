package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"strings"
)

func resourceDeployment() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju deployment.",

		CreateContext: resourceDeploymentCreate,
		ReadContext:   resourceDeploymentRead,
		UpdateContext: resourceDeploymentUpdate,
		DeleteContext: resourceDeploymentDelete,

		Importer: &schema.ResourceImporter{
			// TODO: sync-up with read operation
			//StateContext: resourceDeploymentImporter,
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A custom name for the application deployment. If empty, uses the charm's name.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
			},
			"model": {
				Description: "The name of the model where the charm is to be deployed.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
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
							ForceNew:    true,
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
							Optional:    true,
							Computed:    true,
						},
						"series": {
							Description: "The series on which to deploy.",
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
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

	response, err := client.Deployments.CreateDeployment(&juju.CreateDeploymentInput{
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

	// These values can be computed, and so set from the response.
	d.Set("name", response.AppName)

	charm["revision"] = response.Revision
	charm["series"] = response.Series
	d.Set("charm", []map[string]interface{}{charm})

	id := fmt.Sprintf("%s:%s", modelName, response.AppName)
	d.SetId(id)

	return nil
}

func resourceDeploymentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)
	id := strings.Split(d.Id(), ":")
	modelName, appName := id[0], id[1]
	modelUUID, err := client.Models.ResolveUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	response, err := client.Deployments.ReadDeployment(&juju.ReadDeploymentInput{
		ModelUUID: modelUUID,
		AppName:   appName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	if response == nil {
		return nil
	}

	// TODO: This is a temporary fix to preserve the defined charm channel, as we cannot currently pull this from the API
	// The if exists statement is also required to allow import to function when no exiting data is in the state
	// Remove these lines and uncomment under the next TODO

	var charmList map[string]interface{}
	_, exists := d.GetOk("charm")
	if exists {
		charmList = d.Get("charm").([]interface{})[0].(map[string]interface{})
		charmList["name"] = response.Name
		charmList["revision"] = response.Revision
		charmList["series"] = response.Series
	} else {
		charmList = map[string]interface{}{
			"name":     response.Name,
			"channel":  "latest/stable",
			"revision": response.Revision,
			"series":   response.Series,
		}
	}
	d.Set("charm", []map[string]interface{}{charmList})

	// TODO: Once we can pull the channel from the API, remove the above and uncomment below
	//charmList := []map[string]interface{}{
	//	{
	//		"name":     response.Name,
	//		"channel":  response.Channel,
	//		"revision": response.Revision,
	//		"series":   response.Series,
	//	},
	//}
	//d.Set("charm", charmList)
	d.Set("model", modelName)
	d.Set("name", appName)
	d.Set("units", response.Units)

	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return nil
}

func resourceDeploymentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

// Juju refers to deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func resourceDeploymentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	var diags diag.Diagnostics

	err = client.Deployments.DestroyDeployment(&juju.DestroyDeploymentInput{
		ApplicationName: d.Get("name").(string),
		ModelUUID:       modelUUID,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return diags
}
