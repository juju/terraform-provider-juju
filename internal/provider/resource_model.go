package provider

import (
	"context"
	"encoding/json"
	"strings"

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
		Importer: &schema.ResourceImporter{
			StateContext: resourceModelImporter,
		},

		Schema: map[string]*schema.Schema{
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
				ForceNew:    true,
			},
			"cloud": {
				Description: "JuJu Cloud where the model will operate",
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
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
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	name := d.Get("name").(string)
	controller := d.Get("controller").(string)
	cloud := d.Get("cloud").([]interface{})
	config := d.Get("config").(map[string]interface{})

	modelInfo, err := client.Models.CreateModel(name, controller, cloud, config)
	if err != nil {
		return diag.FromErr(err)
	}

	// TODO: If controller and / or cloud are blank, we should set them to the default (returned in modelInfo)
	// TODO: Should config track all key=value or just those explicitly set?

	d.SetId(modelInfo.UUID)

	return diags
}

func resourceModelRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	uuid := d.Id()
	controllerName, modelInfo, modelConfig, err := client.Models.ReadModel(uuid)
	if err != nil {
		return diag.FromErr(err)
	}

	cloudList := []map[string]interface{}{{
		"name":   strings.TrimPrefix(modelInfo.CloudTag, juju.PrefixCloud),
		"region": modelInfo.CloudRegion},
	}

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

	// Only read model config that is tracked in Terraform
	config := d.Get("config").(map[string]interface{})
	for k := range config {
		if value, exists := modelConfig[k]; exists {
			var serialised string
			switch value.(type) {
			// TODO: review for other possible types
			case bool:
				b, err := json.Marshal(value)
				if err != nil {
					return diag.FromErr(err)
				}
				serialised = string(b)
			default:
				serialised = value.(string)
			}

			config[k] = serialised
		}
	}
	if err = d.Set("config", config); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceModelUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	if d.HasChange("config") {
		modelUUID := d.Id()

		oldConfig, newConfig := d.GetChange("config")
		oldConfigMap := oldConfig.(map[string]interface{})
		newConfigMap := newConfig.(map[string]interface{})

		var unsetConfigKeys []string
		for k := range oldConfigMap {
			if _, ok := newConfigMap[k]; !ok {
				unsetConfigKeys = append(unsetConfigKeys, k)
			}
		}

		err := client.Models.UpdateModel(modelUUID, newConfigMap, unsetConfigKeys)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return diags
}

// Juju refers to model deletion as "destroy" so we call the Destroy function of our client here rather than delete
// This function remains named Delete for parity across the provider and to stick within terraform naming conventions
func resourceModelDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelUUID := d.Id()

	err := client.Models.DestroyModel(modelUUID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}

func resourceModelImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*juju.Client)

	//d.Id() here is the last argument passed to the `terraform import juju_model.RESOURCE_NAME MODEL_NAME` command
	//because we import based on model name we load it into `modelName` here for clarity
	modelName := d.Id()

	model, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return nil, err
	}

	if err = d.Set("name", model.Name); err != nil {
		return nil, err
	}
	d.SetId(model.UUID)

	return []*schema.ResourceData{d}, nil
}
