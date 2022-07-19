package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/juju/terraform-provider-juju/internal/juju"
)

func resourceOffer() *schema.Resource {
	return &schema.Resource{
		// This description is used by the documentation generator and the language server.
		Description: "A resource that represent a Juju Offer.",

		CreateContext: resourceOfferCreate,
		ReadContext:   resourceOfferRead,
		DeleteContext: resourceOfferDelete,

		Schema: map[string]*schema.Schema{
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"name": {
				Description: "The name of the offer.",
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
			},
			"application_name": {
				Description: "The name of the application.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"endpoint": {
				Description: "The endpoint name.",
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
			},
			"url": {
				Description: "The offer URL.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func resourceOfferCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics
	modelName := d.Get("model").(string)
	modelUUID, err := client.Models.ResolveModelUUID(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	result, errs := client.Offers.CreateOffer(&juju.CreateOfferInput{
		ModelName:       modelName,
		ModelUUID:       modelUUID,
		Name:            d.Get("name").(string),
		ApplicationName: d.Get("application_name").(string),
		Endpoint:        d.Get("endpoint").(string),
	})
	if errs != nil {
		if len(errs) == 1 {
			return diag.FromErr(errs[0])
		} else {
			for _, v := range errs {
				diags = append(diags, diag.FromErr(v)...)
			}
			return diags
		}
	}

	if err = d.Set("url", result.OfferURL); err != nil {
		return diag.FromErr(err)
	}

	//TODO: check that a URL is unique
	d.SetId(result.OfferURL)

	return diags
}

func resourceOfferRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// TODO: Add client function to handle the appropriate JuJu API Facade Endpoint
	return diag.Errorf("not implemented")
}

func resourceOfferDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	err := client.Offers.DestroyOffer(&juju.DestroyOfferInput{
		OfferURL: d.Get("url").(string),
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return diags
}
