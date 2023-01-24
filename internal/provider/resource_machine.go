package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceMachine() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represents a Juju machine deployment. Refer to the juju add-machine CLI command for more information and limitations.",
		
		CreateContext: resourceMachineCreate,
		ReadContext: resourceMachineRead,
		UpdateContext: resourceMachineUpdate,
		DeleteContext: resourceMachineDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The Juju model in which to add a new machine.",
				Type: schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"no-browser-login": {
				Description: "Do not use web browser for authentication",
				Type: schema.TypeBool,
				Optional: true,
				Default: false,
			},
			"constraints": {
				Description: "Machine constraints that overwrite those available from 'juju get-model-constraints' and provider's defaults.",
				Type: schema.TypeString,
				Optional: true,
				Default: "",
				ForceNew: true,
			},
			"disks": {
				Description: "Storage constraints for disks to attach to the machine(s).",
				Type: schema.TypeString,
				Optional: true,
				Default: "",
				ForceNew: true,
			},
			"series": {
				Description: "The operating system series to install on the new machine(s).",
				Type: schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"machineId": {
				Description: "The id of the machine Juju creates.",
				Type: schema.TypeString,
				Computed: true,
			},
		}
	}
}

func resourceApplicationCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	noBrowserLogin := d.Get("no-browser-login").(bool)
	constraints := d.Get("constraints").(string)
	disks := d.Get("disks").(string)
	series := d.Get("series").(string)

	response, err := client.Machines.CreateMachine(&juju.CreateMachineInput{
		NoBrowserLogin: noBrowserLogin,
		Constraints:    constraints,
		ModelUUID:      modelUUID,
		Disks:          disks,
		Series:         series,
	})

	if err != nil {
		return diag.FromErr(err)
	}
}