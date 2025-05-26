// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/juju/juju/core/constraints"
)

// This file defines a custom type and value for handling constraints in a
// Terraform provider. The CustomConstraintsType extends the StringType to
// provide custom functionality for parsing and comparing constraints strings.
// This ensures that a replace is triggered only if the constraints string
// changes in a way which is not just formatting.
// This follows https://developer.hashicorp.com/terraform/plugin/framework/handling-data/types/custom

var _ basetypes.StringTypable = CustomConstraintsType{}

// CustomConstraintsType is a custom type for handling constraints in a
// Terraform provider. It extends the StringType to provide custom
// functionality for parsing and comparing constraints strings.
type CustomConstraintsType struct {
	basetypes.StringType
}

// Equal checks if the CustomConstraintsType is equal to another attr.Type.
func (t CustomConstraintsType) Equal(o attr.Type) bool {
	other, ok := o.(CustomConstraintsType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

// String returns a string representation of the CustomConstraintsType.
func (t CustomConstraintsType) String() string {
	return "CustomConstraintsType"
}

// ValueFromString converts a StringValue to a CustomConstraintsValue.
func (t CustomConstraintsType) ValueFromString(ctx context.Context, in basetypes.StringValue) (basetypes.StringValuable, diag.Diagnostics) {
	value := CustomConstraintsValue{
		StringValue: in,
	}
	return value, nil
}

// ValueFromTerraform converts a tftypes.Value to a CustomConstraintsValue.
func (t CustomConstraintsType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
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

// ValueType returns the type of value that this CustomConstraintsType represents.
func (t CustomConstraintsType) ValueType(ctx context.Context) attr.Value {
	// CustomConstraintsValue defined in the value type section
	return CustomConstraintsValue{}
}

var _ basetypes.StringValuable = CustomConstraintsValue{}

// NewCustomConstraintsValue creates a new CustomConstraintsValue from a string.
func NewCustomConstraintsValue(in string) CustomConstraintsValue {
	return CustomConstraintsValue{
		StringValue: basetypes.StringValue(types.StringValue(in)),
	}
}

// CustomConstraintsValue is a custom value type that represents a string
// containing constraints. It extends the StringValue to provide custom
// functionality for parsing and comparing constraints strings.
type CustomConstraintsValue struct {
	basetypes.StringValue
	// ... potentially other fields ...
}

// Equal checks if the CustomConstraintsValue is equal to another attr.Value.
func (v CustomConstraintsValue) Equal(o attr.Value) bool {
	other, ok := o.(CustomConstraintsValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

// Type returns the CustomConstraintsType for this value.
func (v CustomConstraintsValue) Type(ctx context.Context) attr.Type {
	// CustomConstraintsType defined in the schema type section
	return CustomConstraintsType{}
}

var _ basetypes.StringValuableWithSemanticEquals = CustomConstraintsValue{}

// StringSemanticEquals checks if the CustomConstraintsValue is semantically
// equal to another basetypes.StringValuable. It parses the constraints strings
// and compares them for equality, allowing for normalization of the constraints
// before comparison.
// This is useful for ensuring that different representations of the same
// constraints (e.g., different order of constraints) are considered equal.
func (v CustomConstraintsValue) StringSemanticEquals(ctx context.Context, newValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	// The framework should always pass the correct value type, but always check
	newValue, ok := newValuable.(CustomConstraintsValue)
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

	// Parse and normalize constraints for semantic comparison
	priorConstraints, err := constraints.Parse(v.StringValue.ValueString())
	if err != nil {
		diags.AddError(
			"Constraint Parsing Error",
			fmt.Sprintf("Failed to parse prior constraints: %v", err),
		)
		return false, diags
	}
	newConstraints, err := constraints.Parse(newValue.ValueString())
	if err != nil {
		diags.AddError(
			"Constraint Parsing Error",
			fmt.Sprintf("Failed to parse new constraints: %v", err),
		)
		return false, diags
	}

	// If the constraints are equivalent, keep the prior value
	return newConstraints.String() == priorConstraints.String(), diags
}

// constraintsRequiresReplacefunc checks if the constraints in the plan
// require a resource replacement. It compares the constraints from the
// plan and the state, and sets RequiresReplace to true if they differ.
// It is used to ensure that changes to constraints trigger a resource
// replacement, as constraints are a fundamental part of the resource's
// configuration and cannot be updated in place.
func constraintsRequiresReplacefunc(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.ConfigValue.IsNull() {
		return
	}
	if req.StateValue.IsNull() {
		return
	}

	constraintsFromPlan, err := constraints.Parse(req.StateValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to parse constraints from plan")
		return
	}
	constraintsFromConfig, err := constraints.Parse(req.ConfigValue.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", "Unable to parse constraints from config")
		return
	}

	resp.RequiresReplace = constraintsFromConfig.String() != constraintsFromPlan.String()
}
