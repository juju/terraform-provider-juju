package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Currently offers are handled as a part of the integration resource in order to have parity with the CLI. An alternative considered was to create a resource specifically for managing cross model integrations.
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
			"via": {
				Description: "A comma separated list of CIDRs for outbound traffic.",
				Type:        schema.TypeString,
				Optional:    true,
			},
			"application": {
				Description: "The two applications to integrate.",
				Type:        schema.TypeSet,
				Required:    true,
				MaxItems:    2,
				MinItems:    2,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Description: "The name of the application.",
							Type:        schema.TypeString,
							Optional:    true,
						},
						"endpoint": {
							Description: "The endpoint name.",
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
						},
						//TODO: find an alternative to setting Computed: true in `offer_url`
						//`offer_url` has the property `Computed` set to true even though it will never be computed.
						//This is due to an issue with the plugin-sdk/v2 and `schema.TypeSet` meaning that a plan will always show needed changes despite the read op storing the correct state
						"offer_url": {
							Description: "The URL of a remote application.",
							Type:        schema.TypeString,
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
	endpoints, offerURL, err := parseEndpoints(apps)
	if err != nil {
		return diag.FromErr(err)
	}
	if len(endpoints) == 0 {
		return diag.Errorf("you must provide at least one local application")
	}
	var offerResponse = &juju.ConsumeRemoteOfferResponse{}
	if offerURL != nil {
		offerResponse, err = client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
			ModelUUID: modelUUID,
			OfferURL:  *offerURL,
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if offerResponse.SAASName != "" {
		endpoints = append(endpoints, offerResponse.SAASName)
	}

	viaCIDRs := d.Get("via").(string)
	response, err := client.Integrations.CreateIntegration(&juju.IntegrationInput{
		ModelUUID: modelUUID,
		Endpoints: endpoints,
		ViaCIDRs:  viaCIDRs,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	applications := parseApplications(response.Applications)

	id := generateID(modelName, response.Applications)
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

	response, err := client.Integrations.ReadIntegration(int)
	if err != nil {
		return diag.FromErr(err)
	}

	applications := parseApplications(response.Applications)

	if err := d.Set("model", modelName); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("application", applications); err != nil {
		return diag.FromErr(err)
	}

	return diag.Diagnostics{}
}

func resourceIntegrationUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	var old, new interface{}
	var oldEndpoints, endpoints []string
	var oldOfferURL, offerURL *string

	if d.HasChange("application") {
		old, new = d.GetChange("application")
		oldEndpoints, oldOfferURL, err = parseEndpoints(old.(*schema.Set).List())
		if err != nil {
			return diag.FromErr(err)
		}
		endpoints, offerURL, err = parseEndpoints(new.(*schema.Set).List())
		if err != nil {
			return diag.FromErr(err)
		}
	}

	var offerResponse *juju.ConsumeRemoteOfferResponse
	//check if the offer url is present and is not the same as before the change
	if oldOfferURL != offerURL && !(oldOfferURL == nil && offerURL == nil) {
		if oldOfferURL != nil {
			//destroy old offer
			errs := client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
				ModelUUID: modelUUID,
				OfferURL:  *oldOfferURL,
			})
			if len(errs) > 0 {
				for _, v := range errs {
					diags = append(diags, diag.FromErr(v)...)
				}
				return diags
			}
		}
		if offerURL != nil {
			offerResponse, err = client.Offers.ConsumeRemoteOffer(&juju.ConsumeRemoteOfferInput{
				ModelUUID: modelUUID,
				OfferURL:  *offerURL,
			})
			if err != nil {
				return diag.FromErr(err)
			}
			endpoints = append(endpoints, offerResponse.SAASName)
		}
	}

	viaCIDRs := d.Get("via").(string)
	input := &juju.UpdateIntegrationInput{
		ModelUUID:    modelUUID,
		ID:           d.Id(),
		Endpoints:    endpoints,
		OldEndpoints: oldEndpoints,
		ViaCIDRs:     viaCIDRs,
	}

	response, err := client.Integrations.UpdateIntegration(input)
	if err != nil {
		return diag.FromErr(err)
	}

	applications := parseApplications(response.Applications)

	id := generateID(modelName, response.Applications)
	if err := d.Set("application", applications); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id)

	return diags
}

func resourceIntegrationDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	apps := d.Get("application").(*schema.Set).List()
	endpoints, offer, err := parseEndpoints(apps)
	if err != nil {
		return diag.FromErr(err)
	}

	//If one of the endpoints is an offer then we need to remove the remote offer rather than destroying the integration
	if offer != nil {
		errs := client.Offers.RemoveRemoteOffer(&juju.RemoveRemoteOfferInput{
			ModelUUID: modelUUID,
			OfferURL:  *offer,
		})
		if len(errs) > 0 {
			for _, v := range errs {
				diags = append(diags, diag.FromErr(v)...)
			}
			return diags
		}
	} else {
		err = client.Integrations.DestroyIntegration(&juju.IntegrationInput{
			ModelUUID: modelUUID,
			Endpoints: endpoints,
		})
		if err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("")

	return diags
}

func generateID(modelName string, apps []juju.Application) string {
	//In order to generate a stable iterable order we sort the endpoints keys by the role value (provider is always first to match `juju status` output)
	//TODO: verify we always get only 2 endpoints and that the role value is consistent
	keys := make([]int, len(apps))
	for k, v := range apps {
		if v.Role == "provider" {
			keys[0] = k
		} else if v.Role == "requirer" {
			keys[1] = k
		}
	}

	id := modelName
	for _, key := range keys {
		ep := apps[key]
		id = fmt.Sprintf("%s:%s:%s", id, ep.Name, ep.Endpoint)
	}

	return id
}

// This function can be used to parse the terraform data into usable juju endpoints
// it also does some sanity checks on inputs and returns user friendly errors
func parseEndpoints(apps []interface{}) (endpoints []string, offer *string, err error) {
	for _, app := range apps {
		if app == nil {
			return nil, nil, fmt.Errorf("you must provide a non-empty name for each application in an integration")
		}
		a := app.(map[string]interface{})
		name := a["name"].(string)
		offerURL := a["offer_url"].(string)
		endpoint := a["endpoint"].(string)

		if name == "" && offerURL == "" {
			return nil, nil, fmt.Errorf("you must provide one of \"name\" or \"offer_url\"")
		}

		if name != "" && offerURL != "" {
			return nil, nil, fmt.Errorf("you must only provider one of \"name\" or \"offer_url\" and not both")
		}

		if offerURL != "" && endpoint != "" {
			return nil, nil, fmt.Errorf("\"offer_url\" cannot be provided with \"endpoint\"")
		}

		//Here we check if the endpoint is empty and pass just the application name, this allows juju to attempt to infer endpoints
		//If the endpoint is specified we pass the format <applicationName>:<endpoint>
		//first check if we have an offer_url, in this case don't return the endpoint
		if offerURL != "" {
			offer = &offerURL
			continue
		}
		if endpoint == "" {
			endpoints = append(endpoints, name)
		} else {
			endpoints = append(endpoints, fmt.Sprintf("%v:%v", name, endpoint))
		}
	}

	return endpoints, offer, nil
}

func parseApplications(apps []juju.Application) []map[string]interface{} {
	applications := make([]map[string]interface{}, 0, 2)

	for _, app := range apps {
		a := make(map[string]interface{})

		if app.OfferURL != nil {
			a["offer_url"] = app.OfferURL
			a["endpoint"] = ""
			a["name"] = ""
		} else {
			a["endpoint"] = app.Endpoint
			a["name"] = app.Name
			a["offer_url"] = ""
		}
		applications = append(applications, a)
	}

	return applications
}
