// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/provider"
)

func TestResourceKeyValidatorValid(t *testing.T) {
	validResources := make(map[string]string)
	validResources["image1"] = "image/tag:v1.0.0"
	validResources["image2"] = "123.123.123.123:123/image/tag:v1.0.0"
	validResources["image3"] = "your-domain.com/image/tag:v1.1.1-patch1"
	validResources["image4"] = "your_domain/image/tag:patch1"
	validResources["image5"] = "your.domain.com/image/tag:1"
	validResources["image6"] = "27"
	validResources["image7"] = "1"
	ctx := context.Background()

	resourceValidator := provider.StringIsResourceKeyValidator{}
	resourceValue, _ := types.MapValueFrom(ctx, types.StringType, validResources)

	req := validator.MapRequest{
		ConfigValue: resourceValue,
	}

	var resp validator.MapResponse
	resourceValidator.ValidateMap(context.Background(), req, &resp)

	if resp.Diagnostics.HasError() {
		t.Errorf("errors %v", resp.Diagnostics.Errors())
	}
}

func TestResourceKeyValidatorInvalidRevision(t *testing.T) {
	validResources := make(map[string]string)
	validResources["image1"] = "-10"
	validResources["image2"] = "0"
	validResources["image3"] = "10.5"
	validResources["image4"] = "image/tag:"
	validResources["image5"] = ":v1.0.0"
	validResources["image6"] = "your-domain.com"
	ctx := context.Background()

	resourceValidator := provider.StringIsResourceKeyValidator{}
	resourceValue, _ := types.MapValueFrom(ctx, types.StringType, validResources)

	req := validator.MapRequest{
		ConfigValue: resourceValue,
	}

	var resp validator.MapResponse
	resourceValidator.ValidateMap(context.Background(), req, &resp)
	err := "Invalid resource value"
	if c := resp.Diagnostics.ErrorsCount(); c != 6 {
		t.Errorf("expected 6 errors, got %d", c)
	}
	if deets := resp.Diagnostics.Errors()[0].Summary(); err != deets {
		t.Errorf("expected error %q, got %q", err, deets)
	}
}
