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
			"name": {
				Description: "A name for the machine resource in Terraform.",
				Type: schema.TypeString,
				Required: true,
			},
			"model": {
				Description: "The Juju model in which to add a new machine.",
				Type: schema.TypeString,
				Required: true,
				ForceNew: true,
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
				Required: true,
				ForceNew: true,
			},
			"machine_id": {
				Description: "The id of the machine Juju creates.",
				Type: schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceMachineCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}
	name := d.Get("name").(string)
	constraints := d.Get("constraints").(string)
	disks := d.Get("disks").(string)
	series := d.Get("series").(string)

	response, err := client.Machines.CreateMachine(&juju.CreateMachineInput{
		Constraints:    constraints,
		ModelUUID:      modelUUID,
		Disks:          disks,
		Series:         series,
		InstanceId:     name,
	})

	if err != nil {
		return diag.FromErr(err)
	}
	if response.Machines[0].Error != nil {
		return diag.FromErr(err)
	}
	id := fmt.Sprintf("%s:%s", modelName, response.Machines[0].Machine)
	d.Set("machine_id", "machine")
	d.SetId(id)
	return nil
}

func resourceMachineRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*juju.Client)
	id := strings.Split(d.Id(), ":")
	
	if len(id) != 2 {
		return diag.Errorf("unable to parse model and machine ID from provided ID")
	}

	modelName, machineId := id[0], id[1]
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}
	
	response, err := client.Machines.ReadMachine(&juju.ReadMachineInput{
		ModelUUID: modelUUID,
		MachineId: machineId,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	if response == nil {
		return nil
	}

	if err = d.Set("model", modelName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", d.Get("name")); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("series", response.MachineStatus.Series); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("machine_id", machineId); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

func resourceMachineUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	return diags
}

func resourceMachineDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	
	return diags
}