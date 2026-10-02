// Copyright 2025 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"

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

// NewNullCustomConstraintsValue creates a null CustomConstraintsValue.
func NewNullCustomConstraintsValue() CustomConstraintsValue {
	return CustomConstraintsValue{
		StringValue: basetypes.NewStringNull(),
	}
}

// NewCustomConstraintsValue creates a new CustomConstraintsValue from a string.
func NewCustomConstraintsValue(in string) CustomConstraintsValue {
	return CustomConstraintsValue{
		StringValue: basetypes.StringValue(types.StringValue(in)),
	}
}

// NewNormalizedCustomConstraintsValue creates a CustomConstraintsValue and
// normalizes an empty string to null.
func NewNormalizedCustomConstraintsValue(in string) CustomConstraintsValue {
	if in == "" {
		return NewNullCustomConstraintsValue()
	}
	return NewCustomConstraintsValue(in)
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

// StringSemanticEquals compares the constraints Juju returned with the
// constraints the user asked for.
//
// Juju can add constraints from the model, so it may return more than the
// user asked for. That's fine: as long as everything the user asked for is
// there with the same value, they match and we keep what the user wrote.
// For example, the user asks for "mem=4G" and Juju returns
// "cores=1 mem=4096M": that's a match.
//
// "arch" can be missing, because Juju drops it when constraints are updated.
//
// Note: this is only used when reading constraints back from Juju. Deciding
// whether a change to constraints needs a replace uses constraintsEqual.
func (v CustomConstraintsValue) StringSemanticEquals(ctx context.Context, priorValuable basetypes.StringValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics
	// The framework should always pass the correct value type, but always check
	priorValue, ok := priorValuable.(CustomConstraintsValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks. "+
				"Please report this to the provider developers.\n\n"+
				"Expected Value Type: "+fmt.Sprintf("%T", v)+"\n"+
				"Got Value Type: "+fmt.Sprintf("%T", priorValuable),
		)

		return false, diags
	}

	if v.ValueString() == priorValue.ValueString() { // exact match
		return true, diags
	}
	actualMap, priorMap, diags := parseConstraintsPair(v.ValueString(), priorValue.ValueString())
	if diags.HasError() {
		return false, diags
	}

	// Keys only present in actualMap are allowed, e.g. merged model constraints.
	for k, pv := range priorMap {
		av, ok := actualMap[k]
		if !ok {
			if _, optional := optionalConstraintKeys[k]; optional {
				continue
			}
			return false, diags
		}
		if av != pv {
			return false, diags
		}
	}
	return true, diags
}

// constraintsEqual checks whether two constraints strings ask for the same
// constraints. The order and units don't matter, so "mem=4G cores=1" and
// "cores=1 mem=4096M" are equal. Any constraint on only one side makes them
// different, except "arch", which may be missing from either side.
func constraintsEqual(a, b string) (bool, diag.Diagnostics) {
	if a == b { // exact match
		return true, nil
	}
	aMap, bMap, diags := parseConstraintsPair(a, b)
	if diags.HasError() {
		return false, diags
	}

	union := make(map[string]struct{}, len(aMap)+len(bMap))
	for k := range aMap {
		union[k] = struct{}{}
	}
	for k := range bMap {
		union[k] = struct{}{}
	}

	for k := range union {
		av, aOk := aMap[k]
		bv, bOk := bMap[k]
		if !aOk || !bOk { // key only on one side
			if _, optional := optionalConstraintKeys[k]; optional {
				continue
			}
			return false, diags
		}
		if av != bv {
			return false, diags
		}
	}
	return true, diags
}

// optionalConstraintKeys lists constraint keys Juju adds or drops by itself,
// e.g. "arch" is added when an application is deployed and removed when its
// constraints are replaced without it. These should not trigger diffs.
var optionalConstraintKeys = map[string]struct{}{
	"arch": {},
}

// parseConstraintsPair parses two constraints strings and returns each as a
// map of key to normalized value, so they can be compared.
func parseConstraintsPair(a, b string) (map[string]string, map[string]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	aConstraints, err := constraints.Parse(a)
	if err != nil {
		diags.AddError(
			"Constraint Parsing Error",
			fmt.Sprintf("Failed to parse constraints %q: %v", a, err),
		)
		return nil, nil, diags
	}
	bConstraints, err := constraints.Parse(b)
	if err != nil {
		diags.AddError(
			"Constraint Parsing Error",
			fmt.Sprintf("Failed to parse constraints %q: %v", b, err),
		)
		return nil, nil, diags
	}
	return parseConstraintTokens(aConstraints.String()), parseConstraintTokens(bConstraints.String()), diags
}

// parseConstraintTokens converts a constraints string (canonical form
// "key=value key2=value2") into a map. Malformed tokens are ignored.
func parseConstraintTokens(raw string) map[string]string {
	m := make(map[string]string)
	for tok := range strings.FieldsSeq(raw) {
		parts := strings.SplitN(tok, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k == "" || v == "" {
			continue
		}
		m[k] = v
	}
	return m
}

// constraintsRequiresReplacefunc checks if the constraints in the plan
// require a resource replacement. It compares the constraints from the
// plan and the state, and sets RequiresReplace to true if they differ
// (see constraintsEqual).
// It is used to ensure that changes to constraints trigger a resource
// replacement, as constraints are a fundamental part of the resource's
// configuration and cannot be updated in place.
func constraintsRequiresReplacefunc(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
	if req.ConfigValue.IsNull() {
		return
	}
	if req.StateValue.IsNull() {
		return
	}

	equal, diags := constraintsEqual(req.StateValue.ValueString(), req.ConfigValue.ValueString())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	resp.RequiresReplace = !equal
}
