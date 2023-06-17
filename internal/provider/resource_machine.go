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
		ReadContext:   resourceMachineRead,
		DeleteContext: resourceMachineDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Description: "A name for the machine resource in Terraform.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
			},
			"model": {
				Description: "The Juju model in which to add a new machine.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"constraints": {
				Description:   "Machine constraints that overwrite those available from 'juju get-model-constraints' and provider's defaults.",
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "",
				ForceNew:      true,
				ConflictsWith: []string{"ssh_address"},
			},
			"disks": {
				Description:   "Storage constraints for disks to attach to the machine(s).",
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "",
				ForceNew:      true,
				ConflictsWith: []string{"ssh_address"},
			},
			"series": {
				Description:   "The operating system series to install on the new machine(s).",
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"ssh_address"},
			},
			"machine_id": {
				Description: "The id of the machine Juju creates.",
				Type:        schema.TypeString,
				Computed:    true,
				Optional:    false,
				Required:    false,
			},
			"ssh_address": {
				Description: "The user@host directive for manual provisioning an existing machine via ssh. " +
					"Requires public_key_file & private_key_file arguments.",
				Type:          schema.TypeString,
				Optional:      true,
				Default:       "",
				ForceNew:      true,
				RequiredWith:  []string{"public_key_file", "private_key_file"},
				ConflictsWith: []string{"series", "constraints"},
			},
			"public_key_file": {
				Description: "The file path to read the public key from.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				ForceNew:    true,
			},
			"private_key_file": {
				Description: "The file path to read the private key from.",
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				ForceNew:    true,
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
	sshAddress := d.Get("ssh_address").(string)
	publicKeyFile := d.Get("public_key_file").(string)
	privateKeyFile := d.Get("private_key_file").(string)

	if sshAddress == "" {
		// If not provisioning manually, check the series argument.
		// If not set, get it from the model default.
		// TODO (cderici): revisit this part when switched to juju 3.x.
		if series == "" {
			modelInfo, err := client.Models.GetModelByName(modelName)
			if err != nil {
				return diag.FromErr(err)
			}
			series = modelInfo.DefaultSeries
		}
	}

	response, err := client.Machines.CreateMachine(&juju.CreateMachineInput{
		Constraints:    constraints,
		ModelUUID:      modelUUID,
		Disks:          disks,
		Series:         series,
		SSHAddress:     sshAddress,
		PublicKeyFile:  publicKeyFile,
		PrivateKeyFile: privateKeyFile,
	})

	if err != nil {
		return diag.FromErr(err)
	}
	if response.Machine.Error != nil {
		return diag.FromErr(err)
	}
	id := fmt.Sprintf("%s:%s:%s", modelName, response.Machine.Machine, name)
	if err = d.Set("machine_id", response.Machine.Machine); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("series", response.Series); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(id)
	return nil
}

func IsMachineNotFound(err error) bool {
	return strings.Contains(err.Error(), "no status returned for machine")
}

func handleMachineNotFoundError(err error, d *schema.ResourceData) diag.Diagnostics {
	if IsMachineNotFound(err) {
		// Machine manually removed
		d.SetId("")
		return diag.Diagnostics{}
	}

	return diag.FromErr(err)
}

func resourceMachineRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*juju.Client)
	id := strings.Split(d.Id(), ":")

	if len(id) != 3 {
		return diag.Errorf("unable to parse model, machine ID, and name from provided ID")
	}

	modelName, machineId, machineName := id[0], id[1], id[2]
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	response, err := client.Machines.ReadMachine(&juju.ReadMachineInput{
		ModelUUID: modelUUID,
		MachineId: machineId,
	})
	if err != nil {
		return handleMachineNotFoundError(err, d)
	}

	if response == nil {
		return nil
	}

	if err = d.Set("model", modelName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", machineName); err != nil {
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

func resourceMachineDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	client := meta.(*juju.Client)

	id := strings.Split(d.Id(), ":")

	modelName, machineId, _ := id[0], id[1], id[2]
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	err = client.Machines.DestroyMachine(&juju.DestroyMachineInput{
		ModelUUID: modelUUID,
		MachineId: machineId,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return diags
}
