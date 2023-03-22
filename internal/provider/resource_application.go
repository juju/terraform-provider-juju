package provider

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
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
							Description: "The channel to use when deploying a charm. Specified as \\<track>/\\<risk>/\\<branch>.",
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
			"constraints": {
				Description: "Constraints imposed on this application.",
				Type:        schema.TypeString,
				Optional:    true,
				// Set as "computed" to pre-populate and preserve any implicit constraints
				Computed: true,
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
			"placement": {
				Description: "Specify the target location for the application's units",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					oldDirectives := strings.Split(old, ",")
					newDirectives := strings.Split(new, ",")

					sort.Strings(oldDirectives)
					sort.Strings(newDirectives)
					if len(oldDirectives) != len(newDirectives) {
						return false
					}
					var oldPlacements []string
					for index := 0; index < len(oldDirectives); index++ {
						oldPlacement, _ := instance.ParsePlacement(oldDirectives[index])
						if oldPlacement == nil {
							oldPlacements = append(oldPlacements, "")
						} else {
							var oldPlacementBuilder strings.Builder
							if oldPlacement.Scope == "#" {
								splitDirective := strings.Split(oldPlacement.Directive, "/")
								if len(splitDirective) == 3 && splitDirective[1] == "lxd" {
									oldPlacementBuilder.WriteString(splitDirective[1])
									oldPlacementBuilder.WriteString(":")
									oldPlacementBuilder.WriteString(splitDirective[0])
									oldPlacements = append(oldPlacements, oldPlacementBuilder.String())
								} else {
									oldPlacements = append(oldPlacements, oldPlacement.Directive)
								}
							} else if oldPlacement.Scope == "lxd" {
								oldPlacementBuilder.WriteString(oldPlacement.Scope)
								oldPlacementBuilder.WriteString(":")
								oldPlacementBuilder.WriteString(oldPlacement.Directive)
								oldPlacements = append(oldPlacements, oldPlacementBuilder.String())
							} else {
								oldPlacements = append(oldPlacements, oldPlacement.Scope)
							}
						}
					}
					var newPlacements []string
					for index := 0; index < len(newDirectives); index++ {
						newPlacement, _ := instance.ParsePlacement(newDirectives[index])

						if newPlacement == nil {
							newPlacements = append(newPlacements, "")
						} else {
							var newPlacementBuilder strings.Builder
							if newPlacement.Scope == "#" {
								splitDirective := strings.Split(newPlacement.Directive, "/")
								if len(splitDirective) == 3 && splitDirective[1] == "lxd" {
									newPlacementBuilder.WriteString(splitDirective[1])
									newPlacementBuilder.WriteString(":")
									newPlacementBuilder.WriteString(splitDirective[0])
									newPlacements = append(newPlacements, newPlacementBuilder.String())
								} else {
									newPlacements = append(newPlacements, newPlacement.Directive)
								}
							} else if newPlacement.Scope == "lxd" {
								newPlacementBuilder.WriteString(newPlacement.Scope)
								newPlacementBuilder.WriteString(":")
								newPlacementBuilder.WriteString(newPlacement.Directive)
								newPlacements = append(newPlacements, newPlacementBuilder.String())
							} else {
								newPlacements = append(newPlacements, newPlacement.Scope)
							}
						}
					}
					return reflect.DeepEqual(oldPlacements, newPlacements)
				},
			},
			"principal": {
				Description: "Whether this is a Principal application",
				Type:        schema.TypeBool,
				Computed:    true,
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
	placement := d.Get("placement").(string)
	// populate the config parameter
	// terraform only permits a single type. We have to treat
	// strings to have different types
	configField := d.Get("config").(map[string]interface{})

	// if expose is nil, it was not defined
	var expose map[string]interface{} = nil
	exposeField, exposeWasSet := d.GetOk("expose")
	if exposeWasSet {
		// this was set, by default get no fields there
		expose = make(map[string]interface{}, 0)
		aux := exposeField.([]interface{})[0]
		if aux != nil {
			expose = aux.(map[string]interface{})
		}
	}

	revision := charm["revision"].(int)
	if _, exist := d.GetOk("charm.0.revision"); !exist {
		revision = -1
	}

	var parsedConstraints constraints.Value = constraints.Value{}
	readConstraints := d.Get("constraints").(string)
	if readConstraints != "" {
		parsedConstraints, err = constraints.Parse(readConstraints)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	response, err := client.Applications.CreateApplication(&juju.CreateApplicationInput{
		ApplicationName: name,
		ModelUUID:       modelUUID,
		CharmName:       charmName,
		CharmChannel:    channel,
		CharmRevision:   revision,
		CharmSeries:     series,
		Units:           units,
		Config:          configField,
		Constraints:     parsedConstraints,
		Trust:           trust,
		Expose:          expose,
		Placement:       placement,
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

	return resourceApplicationRead(ctx, d, meta)
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

	var charmList map[string]interface{}
	_, exists := d.GetOk("charm")
	if exists {
		charmList = d.Get("charm").([]interface{})[0].(map[string]interface{})
		charmList["name"] = response.Name
		charmList["channel"] = response.Channel
		charmList["revision"] = response.Revision
		charmList["series"] = response.Series
	} else {
		charmList = map[string]interface{}{
			"name":     response.Name,
			"channel":  response.Channel,
			"revision": response.Revision,
			"series":   response.Series,
		}
	}
	if err = d.Set("charm", []map[string]interface{}{charmList}); err != nil {
		return diag.FromErr(err)
	}

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

	if err = d.Set("principal", response.Principal); err != nil {
		return diag.FromErr(err)
	}

	// constraints do not apply to subordinate applications.
	if response.Principal {
		if err = d.Set("constraints", response.Constraints.String()); err != nil {
			return diag.FromErr(err)
		}
	}

	var exposeValue []map[string]interface{} = nil
	if response.Expose != nil {
		exposeValue = []map[string]interface{}{response.Expose}
	}
	if err = d.Set("expose", exposeValue); err != nil {
		return diag.FromErr(err)
	}

	// We focus on those config entries that
	// are not the default value. If the value was the same
	// we ignore it. If no changes were made, jump to the
	// next step.
	// Terraform does not allow to have several types
	// for a schema attribute. We have to transform the string
	// with the potential type we want to compare with.
	previousConfig := d.Get("config").(map[string]interface{})
	// known previously
	// update the values from the previous config
	changes := false
	for k, v := range response.Config {
		// Add if the value has changed from the previous state
		if previousValue, found := previousConfig[k]; found {
			if !juju.EqualConfigEntries(v, previousValue) {
				// remember that this terraform schema type only accepts strings
				previousConfig[k] = v.String()
				changes = true
			}
		} else if !v.IsDefault {
			// Add if the value is not default
			previousConfig[k] = v.String()
			changes = true
		}
	}
	// we only set changes if there is any difference between
	// the previous and the current config values
	if changes {
		if err = d.Set("config", previousConfig); err != nil {
			return diag.FromErr(err)
		}
	}

	if err = d.Set("placement", response.Placement); err != nil {
		return diag.FromErr(err)
	}

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
		oldExpose, newExpose := d.GetChange("expose")
		_, exposeWasSet := d.GetOk("expose")

		expose, unexpose := computeExposeDeltas(oldExpose, newExpose, exposeWasSet)

		updateApplicationInput.Expose = expose
		updateApplicationInput.Unexpose = unexpose
	}

	if d.HasChange("charm.0.revision") {
		revision := d.Get("charm.0.revision").(int)
		updateApplicationInput.Revision = &revision
	}

	if d.HasChange("config") {
		oldConfig, newConfig := d.GetChange("config")
		oldConfigMap := oldConfig.(map[string]interface{})
		newConfigMap := newConfig.(map[string]interface{})
		for k, v := range newConfigMap {
			// we've lost the type of the config value. We compare the string
			// values.
			oldEntry := fmt.Sprintf("%#v", oldConfigMap[k])
			newEntry := fmt.Sprintf("%#v", v)
			if oldEntry != newEntry {
				if updateApplicationInput.Config == nil {
					// initialize just in case
					updateApplicationInput.Config = make(map[string]interface{})
				}
				updateApplicationInput.Config[k] = v
			}
		}
	}

	if d.HasChange("constraints") {
		_, newConstraints := d.GetChange("constraints")
		appConstraints, err := constraints.Parse(newConstraints.(string))
		if err != nil {
			return diag.FromErr(err)
		}
		updateApplicationInput.Constraints = &appConstraints
	}

	err = client.Applications.UpdateApplication(&updateApplicationInput)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceApplicationRead(ctx, d, meta)
}

// computeExposeDeltas computes the differences between the previously
// stored expose value and the current one. The valueSet argument is used
// to indicate whether the value was already set or not in the latest
// read of the plan.
func computeExposeDeltas(oldExpose interface{}, newExpose interface{}, valueSet bool) (map[string]interface{}, []string) {
	var old map[string]interface{} = nil
	var new map[string]interface{} = nil

	if oldExpose != nil {
		aux := oldExpose.([]interface{})
		if len(aux) != 0 && aux[0] != nil {
			old = aux[0].(map[string]interface{})
		}
	}
	if newExpose != nil {
		aux := newExpose.([]interface{})
		if len(aux) != 0 && aux[0] != nil {
			new = aux[0].(map[string]interface{})
		}
	}
	if new == nil && valueSet {
		new = make(map[string]interface{})
	}

	toExpose := make(map[string]interface{})
	toUnexpose := make([]string, 0)
	// if new is nil we unexpose everything
	if new == nil {
		// set all the endpoints to be unexposed
		toUnexpose = append(toUnexpose, old["endpoints"].(string))
		return nil, toUnexpose
	}

	if old != nil {
		old = make(map[string]interface{})
	}

	// if we have new endpoints we have to expose them
	for endpoint, v := range new {
		_, found := old[endpoint]
		if found {
			// this was already set
			// If it is different, unexpose and then expose
			if v != old[endpoint] {
				toUnexpose = append(toUnexpose, endpoint)
				toExpose[endpoint] = v
			}
		} else {
			// this was not set, expose it
			toExpose[endpoint] = v
		}
	}
	return toExpose, toUnexpose
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
