package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceApplication() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju application deployment.",

		CreateContext: resourceApplicationCreate,
		ReadContext:   resourceApplicationRead,
		UpdateContext: resourceApplicationUpdate,
		DeleteContext: resourceApplicationDelete,

		Importer: &schema.ResourceImporter{
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
				Description: "The name of the model where the application is to be deployed.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"charm": {
				Description: "The name of the charm to be installed from Charmhub.",
				Type:        schema.TypeList,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
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
				DefaultFunc: func() (interface{}, error) {
					return make(map[string]interface{}), nil
				},
			},
			"trust": {
				Description: "Set the trust for the application.",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"expose": {
				Description: "Makes an application publicly available over the network",
				Type:        schema.TypeList,
				Optional:    true,
				Default:     nil,
				MaxItems:    1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoints": {
							Description: "Expose only the ports that charms have opened for this comma-delimited list of endpoints",
							Type:        schema.TypeString,
							Default:     "",
							Optional:    true,
						},
						"spaces": {
							Description: "A comma-delimited list of spaces that should be able to access the application ports once exposed.",
							Type:        schema.TypeString,
							Default:     "",
							Optional:    true,
						},
						"cidrs": {
							Description: "A comma-delimited list of CIDRs that should be able to access the application ports once exposed.",
							Type:        schema.TypeString,
							Default:     "",
							Optional:    true,
						},
					},
				},
			},
		},
	}
}

func resourceApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	name := d.Get("name").(string)
	charm := d.Get("charm").([]interface{})[0].(map[string]interface{})
	charmName := charm["name"].(string)
	channel := charm["channel"].(string)
	series := charm["series"].(string)
	units := d.Get("units").(int)
	trust := d.Get("trust").(bool)
	// populate the config parameter
	configField := d.Get("config").(map[string]interface{})
	config := make(map[string]string)
	for k, v := range configField {
		config[k] = v.(string)
	}
	// if expose is nil, it was not defined
	var expose map[string]string = nil
	exposeField, exposeWasSet := d.GetOk("expose")
	if exposeWasSet {
		// this was set, by default get no fields there
		expose = make(map[string]string, 0)
		aux := exposeField.([]interface{})[0]
		if aux != nil {
			expose = aux.(map[string]string)
		}
	}

	revision := charm["revision"].(int)
	if _, exist := d.GetOk("charm.0.revision"); !exist {
		revision = -1
	}

	response, err := client.Applications.CreateApplication(&juju.CreateApplicationInput{
		ApplicationName: name,
		ModelUUID:       modelUUID,
		CharmName:       charmName,
		CharmChannel:    channel,
		CharmRevision:   revision,
		CharmSeries:     series,
		Units:           units,
		Config:          config,
		Trust:           trust,
		Expose:          expose,
	})

	if err != nil {
		return diag.FromErr(err)
	}

	// These values can be computed, and so set from the response.
	if err = d.Set("name", response.AppName); err != nil {
		return diag.FromErr(err)
	}

	charm["revision"] = response.Revision
	charm["series"] = response.Series
	if err = d.Set("charm", []map[string]interface{}{charm}); err != nil {
		return diag.FromErr(err)
	}

	id := fmt.Sprintf("%s:%s", modelName, response.AppName)
	d.SetId(id)

	return nil
}

func resourceApplicationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)
	id := strings.Split(d.Id(), ":")
	//If importing with an incorrect ID we need to catch and provide a user-friendly error
	if len(id) != 2 {
		return diag.Errorf("unable to parse model and application name from provided ID")
	}
	modelName, appName := id[0], id[1]
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	response, err := client.Applications.ReadApplication(&juju.ReadApplicationInput{
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
	if err = d.Set("charm", []map[string]interface{}{charmList}); err != nil {
		return diag.FromErr(err)
	}

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
	if err = d.Set("model", modelName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", appName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("units", response.Units); err != nil {
		return diag.FromErr(err)
	}

	if err = d.Set("trust", response.Trust); err != nil {
		return diag.FromErr(err)
	}

	// TODO: (2022-08-12) The information contained in the deployment plan
	// does not have to be exactly the same returned by Juju.
	// Operations such as expose or config, will return more
	// information than the one required for the plan.
	// Additional logic has to be added here to ignore those
	// fields returned by Juju not addressed in the deployment
	// plan.

	// exposeValue := []map[string]interface{}{response.Expose}
	// if err = d.Set("expose", exposeValue); err != nil {
	// 	return diag.FromErr(err)
	// }

	// if err = d.Set("config", response.Config); err != nil {
	// 	return diag.FromErr(err)
	// }

	return nil
}

func resourceApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	appName := d.Get("name").(string)
	modelName := d.Get("model").(string)
	modelInfo, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}
	updateApplicationInput := juju.UpdateApplicationInput{
		ModelUUID: modelInfo.UUID,
		ModelType: modelInfo.Type,
		AppName:   appName,
	}

	if d.HasChange("units") {
		units := d.Get("units").(int)
		updateApplicationInput.Units = &units
	}

	if d.HasChange("trust") {
		trust := d.Get("trust").(bool)
		updateApplicationInput.Trust = &trust
	}

	if d.HasChange("expose") {
		expose, exposeWasSet := d.GetOk("expose")
		if exposeWasSet {
			if expose.([]interface{})[0] == nil {
				// no params in expose
				updateApplicationInput.Expose = make(map[string]interface{})
			} else {
				// expose has params
				updateApplicationInput.Expose = expose.([]interface{})[0].(map[string]interface{})
			}
		} else {
			// if there is a change and we have no expose, we have
			// to unexpose
			updateApplicationInput.Unexpose = true
			updateApplicationInput.Expose = nil
		}
	}

	if d.HasChange("charm.0.revision") {
		revision := d.Get("charm.0.revision").(int)
		updateApplicationInput.Revision = &revision
	}

	if d.HasChange("config") {
		config := d.Get("config").(map[string]interface{})
		updateApplicationInput.Config = config
	}

	err = client.Applications.UpdateApplication(&updateApplicationInput)
	if err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// Juju refers to deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func resourceApplicationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	var diags diag.Diagnostics

	err = client.Applications.DestroyApplication(&juju.DestroyApplicationInput{
		ApplicationName: d.Get("name").(string),
		ModelUUID:       modelUUID,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return diags
}
