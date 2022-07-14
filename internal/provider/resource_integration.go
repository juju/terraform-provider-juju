package provider

import (
	"context"
	"fmt"
	"strings"

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

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"application": {
				Description: "The two applications to integrate.",
				Type:        schema.TypeSet,
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
							Default:     nil,
							Optional:    true,
							Computed:    true,
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

	apps := d.Get("application").(*schema.Set).List()
	endpoints, err := parseEndpoints(apps)
	if err != nil {
		return diag.FromErr(err)
	}

	resultEndpoints, err := client.Integrations.CreateIntegration(&juju.IntegrationInput{
		ModelUUID: modelUUID,
		Endpoints: endpoints,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	applications := []map[string]interface{}{}

	for key, val := range resultEndpoints {
		applications = append(applications, map[string]interface{}{
			"name":     key,
			"endpoint": val.Name,
		})
	}

	id := generateID(modelName, resultEndpoints)
	if err := d.Set("application", applications); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id)

	return diag.Diagnostics{}
}

func resourceIntegrationRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*juju.Client)

	id := strings.Split(d.Id(), ":")

	modelName := id[0]

	modelID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	int := &juju.IntegrationInput{
		ModelUUID: modelID,
		Endpoints: []string{
			fmt.Sprintf("%v:%v", id[1], id[2]),
			fmt.Sprintf("%v:%v", id[3], id[4]),
		},
	}

	integrations, err := client.Integrations.ReadIntegration(int)
	if err != nil {
		return diag.FromErr(err)
	}

	applications := []map[string]interface{}{}

	for _, ep := range integrations.Endpoints {
		applications = append(applications, map[string]interface{}{
			"name":     ep.ApplicationName,
			"endpoint": ep.Name,
		})
	}

	if err := d.Set("model", modelName); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("application", applications); err != nil {
		return diag.FromErr(err)
	}

	return diag.Diagnostics{}
}

func resourceIntegrationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	var old, new interface{}
	var oldEndpoints, endpoints []string

	if d.HasChange("application") {
		old, new = d.GetChange("application")
		oldEndpoints, err = parseEndpoints(old.(*schema.Set).List())
		if err != nil {
			return diag.FromErr(err)
		}
		endpoints, err = parseEndpoints(new.(*schema.Set).List())
		if err != nil {
			return diag.FromErr(err)
		}
	}

	input := &juju.UpdateIntegrationInput{
		ModelUUID:    modelUUID,
		ID:           d.Id(),
		Endpoints:    endpoints,
		OldEndpoints: oldEndpoints,
	}

	resultEndpoints, err := client.Integrations.UpdateIntegration(input)
	if err != nil {
		return diag.FromErr(err)
	}

	applications := []map[string]interface{}{}

	for key, val := range resultEndpoints {
		applications = append(applications, map[string]interface{}{
			"name":     key,
			"endpoint": val.Name,
		})
	}

	id := generateID(modelName, resultEndpoints)
	if err := d.Set("application", applications); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id)

	return nil
}

func resourceIntegrationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {

	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	apps := d.Get("application").(*schema.Set).List()
	endpoints, err := parseEndpoints(apps)
	if err != nil {
		return diag.FromErr(err)
	}

	err = client.Integrations.DestroyIntegration(&juju.IntegrationInput{
		ModelUUID: modelUUID,
		Endpoints: endpoints,
	})

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diag.Diagnostics{}
}

func generateID(modelName string, endpoints map[string]params.CharmRelation) string {

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

	id := modelName
	for _, key := range keys {
		ep := endpoints[key]
		id = fmt.Sprintf("%s:%s:%s", id, key, ep.Name)
	}

	return id
}

//This function can be used to parse the terraform data into usable juju endpoints
func parseEndpoints(apps []interface{}) ([]string, error) {

	var endpoints []string

	for _, app := range apps {
		if app == nil {
			return nil, fmt.Errorf("you must provide a non-empty name for each application in an integration")
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

	return endpoints, nil
}
