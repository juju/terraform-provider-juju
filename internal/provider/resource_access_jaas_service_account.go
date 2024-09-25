// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
	"strings"

	jimmnames "github.com/canonical/jimm-go-sdk/v3/names"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/juju/names/v5"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &jaasAccessServiceAccountResource{}
var _ resource.ResourceWithConfigure = &jaasAccessServiceAccountResource{}
var _ resource.ResourceWithImportState = &jaasAccessServiceAccountResource{}
var _ resource.ResourceWithConfigValidators = &jaasAccessServiceAccountResource{}

// NewJAASAccessServiceAccountResource returns a new resource for JAAS service account access.
func NewJAASAccessServiceAccountResource() resource.Resource {
	return &jaasAccessServiceAccountResource{genericJAASAccessResource: genericJAASAccessResource{
		targetResource:  serviceAccountInfo{},
		resourceLogName: LogResourceJAASAccessSvcAcc,
	}}
}

type serviceAccountInfo struct{}

// Info implements the [resourceInfo] interface, used to extract the info from a Terraform plan/state.
func (j serviceAccountInfo) Info(ctx context.Context, getter Getter, diag *diag.Diagnostics) (genericJAASAccessData, names.Tag) {
	serviceAccountAccess := jaasAccessServiceAccountResourceServiceAccount{}
	diag.Append(getter.Get(ctx, &serviceAccountAccess)...)
	accessServiceAccount := genericJAASAccessData{
		ID:              serviceAccountAccess.ID,
		Users:           serviceAccountAccess.Users,
		Groups:          serviceAccountAccess.Groups,
		ServiceAccounts: serviceAccountAccess.ServiceAccounts,
		Access:          serviceAccountAccess.Access,
	}
	// When importing, the serviceAccount name will be empty
	var tag names.Tag
	if serviceAccountAccess.ServiceAccountID.ValueString() != "" {
		svcAccID, err := jimmnames.EnsureValidServiceAccountId(serviceAccountAccess.ServiceAccountID.ValueString())
		if err != nil {
			diag.AddError("invalid service account name", err.Error())
			return genericJAASAccessData{}, nil
		}
		tag = jimmnames.NewServiceAccountTag(svcAccID)
	}
	return accessServiceAccount, tag
}

// Save implements the [resourceInfo] interface, used to save info on Terraform's state.
func (j serviceAccountInfo) Save(ctx context.Context, setter Setter, info genericJAASAccessData, tag names.Tag) diag.Diagnostics {
	// Do the reverse of what we did in Info and strip the @serviceaccount suffix.
	svcAccID := strings.TrimSuffix(tag.Id(), "@serviceaccount")
	serviceAccountAccess := jaasAccessServiceAccountResourceServiceAccount{
		ServiceAccountID: basetypes.NewStringValue(svcAccID),
		ID:               info.ID,
		Users:            info.Users,
		Groups:           info.Groups,
		ServiceAccounts:  info.ServiceAccounts,
		Access:           info.Access,
	}
	return setter.Set(ctx, serviceAccountAccess)
}

// ImportHint implements [resourceInfo] and provides a hint to users on the import string format.
func (j serviceAccountInfo) ImportHint() string {
	return "<service-account-id>:<access-level>"
}

// TagFromID validates the id to be a valid service account ID
// and returns a service account tag.
func (j serviceAccountInfo) TagFromID(id string) (names.Tag, error) {
	if !jimmnames.IsValidServiceAccountId(id) {
		return nil, errors.New("invalid model ID")
	}
	return jimmnames.NewServiceAccountTag(id), nil
}

type jaasAccessServiceAccountResource struct {
	genericJAASAccessResource
}

type jaasAccessServiceAccountResourceServiceAccount struct {
	ServiceAccountID types.String `tfsdk:"service_account_id"`
	Users            types.Set    `tfsdk:"users"`
	ServiceAccounts  types.Set    `tfsdk:"service_accounts"`
	Groups           types.Set    `tfsdk:"groups"`
	Access           types.String `tfsdk:"access"`

	// ID required for imports
	ID types.String `tfsdk:"id"`
}

// Metadata returns metadata about the JAAS serviceAccount access resource.
func (a *jaasAccessServiceAccountResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_jaas_access_service_account"
}

// Schema defines the schema for the JAAS serviceAccount access resource.
func (a *jaasAccessServiceAccountResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	attributes := a.partialAccessSchema()
	attributes["service_account_id"] = schema.StringAttribute{
		Description: "The ID of the service account for access management. If this is changed the resource will be deleted and a new resource will be created.",
		Required:    true,
		Validators: []validator.String{
			ValidatorMatchString(func(s string) bool {
				return jimmnames.IsValidServiceAccountId(s + "@serviceaccount")
			}, "serviceAccount must be a valid user ID i.e. a string starting/ending with an alphanumeric and containing alphanumerics and/or limited special characters."),
		},
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
	}
	schema := schema.Schema{
		Description: "A resource that represents access to a service account when using JAAS.",
		Attributes:  attributes,
	}
	resp.Schema = schema
}
