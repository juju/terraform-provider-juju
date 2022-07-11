package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceIntegration() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju Integration.",

		CreateContext: resourceIntegrationCreate,
		ReadContext:   resourceIntegrationRead,
		UpdateContext: resourceIntegrationUpdate,
		DeleteContext: resourceIntegrationDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"application": {
				Description: "The two applications to integrate.",
				Type:        schema.TypeList,
				Required:    true,
				MaxItems:    2,
				MinItems:    2,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": &schema.Schema{
							Description: "The name of the application.",
							Type:        schema.TypeString,
							Required:    true,
						},
						"endpoint": {
							Description: "The endpoint name.",
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

func resourceIntegrationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	var endpoints []string

	apps := d.Get("application").([]interface{})
	for _, app := range apps {
		if app == nil {
			return diag.Errorf("you must provide a non-empty name for each application in an integration")
		}

		//Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		//If the endpoint is specifed we pass the format <applicationName>:<endpoint>
		a := app.(map[string]interface{})
		if a["endpoint"].(string) == "" {
			endpoints = append(endpoints, a["name"].(string))
		} else {
			endpoints = append(endpoints, fmt.Sprintf("%v:%v", a["name"].(string), a["endpoint"].(string)))
		}
	}

	integration, err := client.Integrations.CreateIntegration(&juju.CreateIntegrationInput{
		ModelUUID: modelUUID,
		Endpoints: endpoints,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	id := fmt.Sprintf("%s:%s", modelName, generateID(integration.Endpoints))

	d.SetId(id)

	return diag.Diagnostics{}
}

func resourceIntegrationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceIntegrationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceIntegrationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func generateID(endpoints map[string]params.CharmRelation) string {

	//In order to generate a stable iterable order we sort the endpoints keys by the role value (requirer is always first)
	//TODO: verify we always get only 2 endpoints and that the role value is consistent
	keys := make([]string, len(endpoints))
	for k, v := range endpoints {
		if v.Role == "requirer" {
			keys[0] = k
		} else if v.Role == "provider" {
			keys[1] = k
		}
	}

	var id string
	for _, key := range keys {
		ep := endpoints[key]
		if id != "" {
			id = fmt.Sprintf("%s:", id)
		}
		id = fmt.Sprintf("%s%s:%s", id, key, ep.Name)
	}

	return id
}
