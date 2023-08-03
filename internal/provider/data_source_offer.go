package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

func dataSourceOffer() *schema.Resource {
	return &schema.Resource{
		Description: "A data source representing a Juju Offer.",
		ReadContext: dataSourceOfferRead,
		Schema: map[string]*schema.Schema{
			"url": {
				Description: "The offer URL.",
				Type:        schema.TypeString,
				Required:    true,
			},
			"model": {
				Description: "The name of the model to operate in.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"name": {
				Description: "The name of the offer.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"application_name": {
				Description: "The name of the application.",
				Type:        schema.TypeString,
				Computed:    true,
			},
			"endpoint": {
				Description: "The endpoint name.",
				Type:        schema.TypeString,
				Computed:    true,
			},
		},
	}
}

func dataSourceOfferRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	offerUrl := d.Get("url").(string)

	offer, err := client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: offerUrl,
	})

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(offer.OfferURL)
	if err = d.Set("url", offer.OfferURL); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("model", offer.ModelName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", offer.Name); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("application_name", offer.ApplicationName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("endpoint", offer.Endpoint); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
