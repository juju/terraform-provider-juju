package provider

import (
	"context"
	"fmt"
	"strings"

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
			},
			"auth_type": {
				Description: "Credential authorization type",
				Type:        schema.TypeString,
				Required:    true,
			},
			"attributes": {
				Description: "Credential attributes accordingly to the cloud",
				Type:        schema.TypeMap,
				Required:    true,
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
	credentialName := d.Get("name").(string)

	response, err := client.Credentials.CreateCredential(juju.CreateCredentialInput{
		Attributes: attributes,
		AuthType:   authType,
		CloudList:  cloud,
		Name:       credentialName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	id := fmt.Sprintf("%s:%s", credentialName, response.CloudName)
	d.SetId(id)

	return diags
}

func resourceCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":") // to be improved
	if len(id) != 2 {
		return diag.Errorf("unable to parse credential name and cloud name from provided ID")
	}
	credentialName, cloudName := id[0], id[1]

	response, err := client.Credentials.ReadCredential(credentialName, cloudName)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", response.CloudCredential.Label); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("auth_type", response.CloudCredential.AuthType()); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// TODO
func resourceCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)
	var diags diag.Diagnostics
	err := client.Credentials.UpdateCredential(juju.UpdateCredentialInput{})
	if err != nil {
		return diag.FromErr(err)
	}
	return diags
}

// TODO
func resourceCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	return diags
}

// TODO
func resourceCredentialImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	return []*schema.ResourceData{d}, nil
}
