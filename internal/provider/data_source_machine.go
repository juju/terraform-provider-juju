package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func dataSourceMachine() *schema.Resource {
	return &schema.Resource{
		Description: "A data source representing a Juju Machine.",
		ReadContext: dataSourceMachineRead,
		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"machine_id": {
				Description: "The Juju id of the machine.",
				Type:        schema.TypeString,
				Required:    true,
			},
		},
	}
}

func dataSourceMachineRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	modelName := d.Get("model").(string)
	machine_id := d.Get("machine_id").(string)

	model, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	machine, err := client.Machines.ReadMachine(&juju.ReadMachineInput{
		ModelUUID: model.UUID,
		MachineId: machine_id,
	})

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(machine.MachineId)
	if err = d.Set("model", model.Name); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("machine_id", machine.MachineId); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
