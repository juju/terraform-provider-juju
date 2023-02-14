package provider

import (
	"context"
	"fmt"
	"strconv"
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
					},
				},
			},
			"attributes": {
				Description: "Credential attributes accordingly to the cloud",
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString, Optional: true},
			},
			"auth_type": {
				Description: "Credential authorization type",
				Type:        schema.TypeString,
				Required:    true,
			},
			"client_credential": {
				Description: "Add credentials to the client",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
			},
			"controller_credential": {
				Description: "Add credentials to the controller",
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
			},
			"name": {
				Description: "The name to be assigned to the credential",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
		},
	}
}

func AttributeEntryToString(input interface{}) string {
	switch t := input.(type) {
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', 0, 64)
	default:
		return input.(string)
	}
}

func resourceCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	attributesRaw := d.Get("attributes").(map[string]interface{})
	authType := d.Get("auth_type").(string)
	clientCredential := d.Get("client_credential").(bool)
	cloud := d.Get("cloud").([]interface{})
	controllerCredential := d.Get("controller_credential").(bool)
	credentialName := d.Get("name").(string)

	attributes := make(map[string]string)
	for key, value := range attributesRaw {
		attributes[key] = AttributeEntryToString(value)
	}
	response, err := client.Credentials.CreateCredential(juju.CreateCredentialInput{
		Attributes:           attributes,
		AuthType:             authType,
		ClientCredential:     clientCredential,
		CloudList:            cloud,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	id := fmt.Sprintf("%s:%s:%t:%t", credentialName, response.CloudName, clientCredential, controllerCredential)
	d.SetId(id)

	return diags
}

func resourceCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	if len(id) != 4 {
		return diag.Errorf("unable to parse credential name and cloud name from provided ID")
	}
	credentialName, cloudName, clientCredentialStr, controllerCredentialStr := id[0], id[1], id[2], id[3]

	cloudList := []map[string]interface{}{{
		"name":   cloudName,
	}}
	if err := d.Set("cloud", cloudList); err != nil {
		return diag.FromErr(err)
	}

	clientCredential, controllerCredential, err := convertOptionsBool(clientCredentialStr, controllerCredentialStr)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("client_credential", clientCredential); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("controller_credential", controllerCredential); err != nil {
		return diag.FromErr(err)
	}

	response, err := client.Credentials.ReadCredential(juju.ReadCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", response.CloudCredential.Label); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("auth_type", response.CloudCredential.AuthType()); err != nil {
		return diag.FromErr(err)
	}

	receivedAttributes := response.CloudCredential.Attributes()

	configuredAttributes := d.Get("attributes").(map[string]interface{})
	for configAtr := range configuredAttributes {
		if receivedValue, exists := receivedAttributes[configAtr]; exists {
			configuredAttributes[configAtr] = AttributeEntryToString(receivedValue)
		}
	}
	if err = d.Set("attributes", configuredAttributes); err != nil {
		return diag.FromErr(err)
	}
	return diags
}

func resourceCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)
	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	if len(id) != 4 {
		return diag.Errorf("unable to parse credential name and cloud name from provided ID")
	}
	credentialName, cloudName := id[0], id[1]

	if !d.HasChange("auth_type") && !d.HasChange("client_credential") && !d.HasChange("controller_credential") && !d.HasChange("attributes") {
		// no changes
		return diags
	}

	newAuthType := d.Get("auth_type").(string)
	newClientCredential := d.Get("client_credential").(bool)
	newControllerCredential := d.Get("controller_credential").(bool)
	newAttributes := make(map[string]string)
	attributesRaw := d.Get("attributes").(map[string]interface{})
	for key, value := range attributesRaw {
		newAttributes[key] = AttributeEntryToString(value)
	}

	err := client.Credentials.UpdateCredential(juju.UpdateCredentialInput{
		Attributes:           newAttributes,
		AuthType:             newAuthType,
		ClientCredential:     newClientCredential,
		CloudName:            cloudName,
		ControllerCredential: newControllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	newID := fmt.Sprintf("%s:%s:%t:%t", credentialName, cloudName, newClientCredential, newControllerCredential)
	d.SetId(newID)

	return diags
}

func resourceCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// When removing cloud credential from a controller, Juju performs additional
	// checks to ensure that there are no models using this credential. The provider will not force the removal
	client := meta.(*juju.Client)
	var diags diag.Diagnostics

	id := strings.Split(d.Id(), ":")
	if len(id) != 4 {
		return diag.Errorf("unable to parse credential name and cloud name from provided ID")
	}
	credentialName, cloudName, clientCredentialStr, controllerCredentialStr := id[0], id[1], id[2], id[3]
	clientCredential, controllerCredential, err := convertOptionsBool(clientCredentialStr, controllerCredentialStr)
	if err != nil {
		return diag.FromErr(err)
	}

	err = client.Credentials.DestroyCredential(juju.DestroyCredentialInput{
		ClientCredential:     clientCredential,
		CloudName:            cloudName,
		ControllerCredential: controllerCredential,
		Name:                 credentialName,
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}

func convertOptionsBool(clientCredentialStr, controllerCredentialStr string) (bool, bool, error) {
	clientCredentialBool, err := strconv.ParseBool(clientCredentialStr)
	if err != nil {
		return false, false, fmt.Errorf("unable to parse client credential from provided ID")
	}

	controllerCredentialBool, err := strconv.ParseBool(controllerCredentialStr)
	if err != nil {
		return false, false, fmt.Errorf("unable to parse controller credential from provided ID")
	}

	return clientCredentialBool, controllerCredentialBool, nil
}

// TODO
func resourceCredentialImporter(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	return []*schema.ResourceData{d}, nil
}
