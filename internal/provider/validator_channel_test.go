// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider_test

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/juju/terraform-provider-juju/internal/provider"
)

func TestChannelValidatorValid(t *testing.T) {
	validChannels := []types.String{
		types.StringValue("track/stable"),
		types.StringValue("track/edge/branch"),
		types.StringNull(),
		types.StringUnknown(),
	}

	channelValidator := provider.StringIsChannelValidator{}
	for _, channel := range validChannels {
		req := validator.StringRequest{
			ConfigValue: channel,
		}
		var resp validator.StringResponse
		channelValidator.ValidateString(context.Background(), req, &resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("errors %v", resp.Diagnostics.Errors())
		}
	}
}

func TestChannelValidatorInvalid(t *testing.T) {
	invalidChannels := []struct {
		str types.String
		err string
	}{{
		str: types.StringValue("track"),
		err: "String must conform to track/risk or track/risk/branch, e.g. latest/stable",
	}, {
		str: types.StringValue("edge"),
		err: "String must conform to track/risk or track/risk/branch, e.g. latest/stable",
	}, {
		str: types.StringValue(`track\risk`),
		err: "String must conform to track/risk or track/risk/branch, e.g. latest/stable",
	}, {
		str: types.StringValue(`track/invalidrisk`),
		err: `risk in channel "track/invalidrisk" not valid`,
	}, {
		str: types.StringValue(`track/invalidrisk/branch`),
		err: `risk in channel "track/invalidrisk/branch" not valid`,
	}}

	channelValidator := provider.StringIsChannelValidator{}
	for _, test := range invalidChannels {
		req := validator.StringRequest{
			ConfigValue: test.str,
		}
		var resp validator.StringResponse
		channelValidator.ValidateString(context.Background(), req, &resp)

		if c := resp.Diagnostics.ErrorsCount(); c != 1 {
			t.Errorf("expected one error, got %d", c)
		}
		if deets := resp.Diagnostics.Errors()[0].Detail(); deets != test.err {
			t.Errorf("expected error %q, got %q", test.err, deets)
		}
	}
}
