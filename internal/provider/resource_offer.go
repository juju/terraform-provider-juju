package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	frameworkResSchema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &offerResource{}
var _ resource.ResourceWithConfigure = &offerResource{}
var _ resource.ResourceWithImportState = &offerResource{}

func NewOfferResource() resource.Resource {
	return &offerResource{}
}

type offerResource struct {
	client *juju.Client
}

func (o offerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_offer"
}

func (o offerResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = frameworkResSchema.Schema{
		Description: "A resource that represent a Juju Offer.",
		Attributes: map[string]frameworkResSchema.Attribute{
			"model": frameworkResSchema.StringAttribute{
				Description: "The name of the model to operate in.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": frameworkResSchema.StringAttribute{
				Description: "The name of the offer.",
				Optional:    true,
				Computed:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"application_name": frameworkResSchema.StringAttribute{
				Description: "The name of the application.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"endpoint": frameworkResSchema.StringAttribute{
				Description: "The endpoint name.",
				Required:    true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"url": frameworkResSchema.StringAttribute{
				Description: "The offer URL.",
				Computed:    true,
			},
		},
	}
}

func (o offerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	//TODO implement me
	panic("implement me")
}

func (o offerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	//TODO implement me
	panic("implement me")
}

func (o offerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	//TODO implement me
	panic("implement me")
}

func (o offerResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	//TODO implement me
	panic("implement me")
}

func (c offerResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*juju.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *juju.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	c.client = client
}

func (c offerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func resourceOfferCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics
	modelName := d.Get("model").(string)
	modelInfo, err := client.Models.GetModelByName(modelName)
	if err != nil {
		return diag.FromErr(err)
	}

	modelUUID := modelInfo.UUID
	modelOwner := strings.TrimPrefix(modelInfo.OwnerTag, juju.PrefixUser)

	//here we verify if the name property is set, if not set to the application name
	var offerName string
	name, ok := d.GetOk("name")
	if ok {
		offerName = name.(string)
	} else {
		offerName = d.Get("application_name").(string)
	}

	result, errs := client.Offers.CreateOffer(&juju.CreateOfferInput{
		ModelName:       modelName,
		ModelUUID:       modelUUID,
		ModelOwner:      modelOwner,
		Name:            offerName,
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

	//in case the name was unset by user we make sure it's set here
	if err = d.Set("name", result.Name); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("url", result.OfferURL); err != nil {
		return diag.FromErr(err)
	}

	//TODO: check that a URL is unique
	d.SetId(result.OfferURL)

	return diags
}

func IsOfferNotFound(err error) bool {
	return strings.Contains(err.Error(), "expected to find one result for url")
}

func handleOfferNotFoundError(err error, d *schema.ResourceData) diag.Diagnostics {
	if IsOfferNotFound(err) {
		// Offer manually removed
		d.SetId("")
		return diag.Diagnostics{}
	}

	return diag.FromErr(err)
}

func resourceOfferRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*juju.Client)

	var diags diag.Diagnostics

	result, err := client.Offers.ReadOffer(&juju.ReadOfferInput{
		OfferURL: d.Id(),
	})
	if err != nil {
		return handleOfferNotFoundError(err, d)
	}

	if err = d.Set("model", result.ModelName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("name", result.Name); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("application_name", result.ApplicationName); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("endpoint", result.Endpoint); err != nil {
		return diag.FromErr(err)
	}
	if err = d.Set("url", result.OfferURL); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(result.OfferURL)

	return diags
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
