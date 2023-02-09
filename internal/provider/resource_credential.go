package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceCredential() *schema.Resource {
	return &schema.Resource{
		Description: "A resource that represent a credential for a cloud.",

		CreateContext: resourceCredentialCreate,
		ReadContext:   resourceCredentialRead,
		UpdateContext: resourceCredentialUpdate,
		DeleteContext: resourceCredentialDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceCredentialImporter,
		},

		Schema: map[string]*schema.Schema{
			"cloud": {
				Description: "JuJu Cloud where the credentials will be used to access",
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
			"name": {
				Description: "The name to be assigned to the credential",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"auth_type": {
				Description: "Credential authorization type",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"attributes": {
				Description: "Credential attributes accordingly to the cloud",
				Type:        schema.TypeMap,
				Required:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	attributes := d.Get("attributes").(map[string]string)
	authType := d.Get("auth_type").(string)
	cloud := d.Get("cloud").([]interface{})
	name := d.Get("name").(string)

	response, err := client.Credentials.CreateCredential(juju.CreateCredentialInput{
		Attributes: attributes,
		AuthType:   authType,
		CloudList:  cloud,
		Name:       name,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// TODO
func resourceCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	uuid := d.Id()
	response, err := client.Credentials.ReadCredential(uuid)
	if err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// TODO
func resourceCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics
	anyChange := false

	return diags
}

// TODO
func resourceCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	modelUUID := d.Id()

	return diags
}

// TODO
func resourceCredentialImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*juju.Client)

	return []*schema.ResourceData{d}, nil
}
