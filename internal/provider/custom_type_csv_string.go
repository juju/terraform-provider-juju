// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ basetypes.StringTypable = CustomCommaDelimitedStringType{}

// CustomCommaDelimitedStringType is a custom type for handling strings containing comma separated values
// in a Terraform provider. It extends the StringType to provide custom
// functionality for parsing and comparing comma separated strings.
type CustomCommaDelimitedStringType struct {
	basetypes.StringType
}

// Equal checks if the CustomCSVStringType is equal to another attr.Type.
func (t CustomCommaDelimitedStringType) Equal(o attr.Type) bool {
	other, ok := o.(CustomCommaDelimitedStringType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

// String returns a string representation of the CustomCSVStringType.
func (t CustomCommaDelimitedStringType) String() string {
	return "CustomCommaDelimitedStringType"
}

// ValueFromString converts a StringValue to a CustomCommaDelimitedStringValue.
func (t CustomCommaDelimitedStringType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	if in.ValueString() != "" {
		return NewCustomCommaDelimitedStringValue(in.ValueString()), nil
	} else {
		return CustomCommaDelimitedStringValue{
			StringValue: in,
		}, nil
	}
}

// ValueFromTerraform converts a tftypes.Value to a CustomCommaDelimitedStringValue.
func (t CustomCommaDelimitedStringType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}
	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}
	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}
	return stringValuable, nil
}

// ValueType returns the type of value that this CustomCSVStringType represents.
func (t CustomCommaDelimitedStringType) ValueType(ctx context.Context) attr.Value {
	// CustomCommaDelimitedStringValue defined in the value type section
	return CustomCommaDelimitedStringValue{}
}

var _ basetypes.StringValuable = CustomCommaDelimitedStringValue{}

// NewCustomCommaDelimitedStringValue creates a new CustomCommaDelimitedStringValue from a string.
func NewCustomCommaDelimitedStringValue(in string) CustomCommaDelimitedStringValue {
	return CustomCommaDelimitedStringValue{
		StringValue: basetypes.StringValue(types.StringValue(in)),
	}
}

// CustomCommaDelimitedStringValue is a custom value type that represents a string
// containing comma delimited values. It extends the StringValue to provide custom
// functionality for comparing comma delimited strings.
type CustomCommaDelimitedStringValue struct {
	basetypes.StringValue
	// ... potentially other fields ...
}

// Equal checks if the CustomCommaDelimitedStringValue is equal to another attr.Value.
func (v CustomCommaDelimitedStringValue) Equal(o attr.Value) bool {
	other, ok := o.(CustomCommaDelimitedStringValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// Type returns the CustomCommaDelimitedStringType for this value.
func (v CustomCommaDelimitedStringValue) Type(ctx context.Context) attr.Type {
	// CustomCommaDelimitedStringType defined in the schema type section
	return CustomCommaDelimitedStringType{}
}

var _ basetypes.StringValuableWithSemanticEquals = CustomCommaDelimitedStringValue{}

// StringSemanticEquals checks if the CustomCommaDelimitedStringValue is semantically
// equal to another basetypes.StringValuable. It compares the contents of
// comma separated string values.
// This is useful for ensuring a different ordering of values in a comma
// separated string does still results in semantically equal strings.
func (v CustomCommaDelimitedStringValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	// The framework should always pass the correct value type, but always check
	newValue, ok := newValuable.(CustomCommaDelimitedStringValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", newValuable),
		)

		return false, diags
	}

	// Split the two strings into tokens and compare the two slices
	leftRaw := v.StringValue.ValueString()
	rightRaw := newValue.ValueString()
	if leftRaw == rightRaw { // exact match
		return true, diags
	}

	leftTokens := strings.Split(leftRaw, ",")
	rightTokens := strings.Split(rightRaw, ",")

	if len(leftTokens) != len(rightTokens) {
		return false, diags
	}

	slices.Sort(leftTokens)
	slices.Sort(rightTokens)

	return slices.Equal(leftTokens, rightTokens), diags
}
