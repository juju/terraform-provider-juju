// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/stretchr/testify/assert"
)

func TestBasicEmailValidation(t *testing.T) {
	testCases := []struct {
		desc    string
		input   string
		matches bool
	}{
		{
			desc:    "With @ symbol is valid",
			input:   "foo@bar",
			matches: true,
		},
		{
			desc:    "Without @ symbol is invalid",
			input:   "foo_bar",
			matches: false,
		},
		{
			desc:    "Require text before @ symbol",
			input:   "@bar",
			matches: false,
		},
		{
			desc:    "Require text after @ symbol",
			input:   "foo@",
			matches: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, basicEmailValidationRe.MatchString(tC.input), tC.matches)
		})
	}
}

func TestAvoidAtSymbolValidation(t *testing.T) {
	testCases := []struct {
		desc    string
		input   string
		matches bool
	}{
		{
			desc:    "Without @ symbol is valid",
			input:   "foo-bar",
			matches: true,
		},
		{
			desc:    "With @ symbol is invalid",
			input:   "foo@bar",
			matches: false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			assert.Equal(t, avoidAtSymbolRe.MatchString(tC.input), tC.matches)
		})
	}
}

// ===============================
// Helpers for jaas resource tests

// newCheckAttribute returns a fetchComputedAttribute object that can be used in tests
// where you want to obtain the value of a computed attributed.
//
// The tag and resourceID fields are empty until this object is passed to a function
// like testAccCheckAttributeNotEmpty.
//
// The relationBuilder parameter allows you to create a custom string
// from the retrieved attribute value that can be used elsewhere in your test.
// The output of this function is stored on the tag field.
func newCheckAttribute(resourceName, attribute string, relationBuilder func(s string) string) fetchComputedAttribute {
	var resourceID string
	var tag string
	return fetchComputedAttribute{
		resourceName:   resourceName,
		attribute:      attribute,
		resourceID:     &resourceID,
		tag:            &tag,
		tagConstructor: relationBuilder,
	}
}

type fetchComputedAttribute struct {
	resourceName   string
	attribute      string
	resourceID     *string
	tag            *string
	tagConstructor func(s string) string
}

// testAccCheckAttributeNotEmpty is used used alongside newCheckAttribute
// to fetch an attribute value and verify that it is not empty.
func testAccCheckAttributeNotEmpty(check fetchComputedAttribute) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		// retrieve the resource by name from state
		rs, ok := s.RootModule().Resources[check.resourceName]
		if !ok {
			return fmt.Errorf("Not found: %s", check.resourceName)
		}

		val, ok := rs.Primary.Attributes[check.attribute]
		if !ok {
			return fmt.Errorf("%s is not set", check.attribute)
		}
		if val == "" {
			return fmt.Errorf("%s is empty", check.attribute)
		}
		if check.resourceID == nil || check.tag == nil {
			return fmt.Errorf("cannot set resource info, nil poiner")
		}
		*check.resourceID = val
		*check.tag = check.tagConstructor(val)
		return nil
	}
}

// testAccCheckJaasResourceAccess verifies whether relations exist
// between the object and target.
// Object and target are expected to be Juju tags of the form <resource-type>:<id>
// Use newCheckAttribute to fetch and format resource tags from computed resources.
func testAccCheckJaasResourceAccess(relation string, object, target *string, expectedAccess bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if object == nil || *object == "" {
			return fmt.Errorf("no object set")
		}
		if target == nil || *target == "" {
			return fmt.Errorf("no target set")
		}
		conn, err := TestClient.Models.GetConnection(nil)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		jc := api.NewClient(juju.JaasConnShim{Connection: conn})
		req := params.ListRelationshipTuplesRequest{
			Tuple: params.RelationshipTuple{
				Object:       *object,
				Relation:     relation,
				TargetObject: *target,
			},
		}
		resp, err := jc.ListRelationshipTuples(&req)
		if err != nil {
			if strings.Contains(err.Error(), "not found") && expectedAccess == false {
				return nil
			}
			return err
		}
		hasAccess := len(resp.Tuples) != 0
		if hasAccess != expectedAccess {
			var accessMsg string
			if expectedAccess {
				accessMsg = "access"
			} else {
				accessMsg = "no access"
			}
			return fmt.Errorf("expected %s for %s as %s to resource (%s), but access is %t", accessMsg, *object, relation, *target, hasAccess)
		}
		return nil
	}
}

func TestDiffStringSets(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		desc     string
		current  []string
		target   []string
		expected []string
	}{
		{
			desc:     "Empty sets",
			current:  []string{},
			target:   []string{},
			expected: []string{},
		},
		{
			desc:     "Identical sets",
			current:  []string{"a", "b"},
			target:   []string{"a", "b"},
			expected: []string{},
		},
		{
			desc:     "Disjoint sets",
			current:  []string{"a", "b"},
			target:   []string{"c", "d"},
			expected: []string{"a", "b"},
		},
		{
			desc:     "Overlapping sets",
			current:  []string{"a", "b", "c"},
			target:   []string{"b", "c", "d"},
			expected: []string{"a"},
		},
		{
			desc:     "Subset",
			current:  []string{"a", "b"},
			target:   []string{"a", "b", "c"},
			expected: []string{},
		},
		{
			desc:     "Superset",
			current:  []string{"a", "b", "c"},
			target:   []string{"a", "b"},
			expected: []string{"c"},
		},
	}

	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			currentSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, tC.current)
			targetSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, tC.target)
			var diags diag.Diagnostics

			result := diffStringSets(currentSet, targetSet, &diags)

			assert.False(t, diags.HasError())

			var resultSlice []string
			result.ElementsAs(ctx, &resultSlice, false)
			assert.ElementsMatch(t, tC.expected, resultSlice)
		})
	}
}

func TestDiffStringSets_Validation(t *testing.T) {
	ctx := context.Background()

	t.Run("Invalid current set type", func(t *testing.T) {
		currentSet, _ := basetypes.NewSetValueFrom(ctx, types.Int64Type, []int64{1, 2})
		targetSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, []string{"a"})
		var diags diag.Diagnostics

		diffStringSets(currentSet, targetSet, &diags)

		assert.True(t, diags.HasError())
		assert.Equal(t, "Internal Error", diags[0].Summary())
		assert.Equal(t, "Mismatched set element types for set diffing", diags[0].Detail())
	})

	t.Run("Invalid target set type", func(t *testing.T) {
		currentSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, []string{"a"})
		targetSet, _ := basetypes.NewSetValueFrom(ctx, types.Int64Type, []int64{1, 2})
		var diags diag.Diagnostics

		diffStringSets(currentSet, targetSet, &diags)

		assert.True(t, diags.HasError())
		assert.Equal(t, "Internal Error", diags[0].Summary())
		assert.Equal(t, "Mismatched set element types for set diffing", diags[0].Detail())
	})

	t.Run("Nil current set type", func(t *testing.T) {
		currentSet := types.SetNull(nil)
		targetSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, []string{"a"})
		var diags diag.Diagnostics

		diff := diffStringSets(currentSet, targetSet, &diags)

		assert.False(t, diags.HasError())
		var resultSlice []string
		diff.ElementsAs(ctx, &resultSlice, false)
		assert.ElementsMatch(t, []string{}, resultSlice)
	})

	t.Run("Nil target set type", func(t *testing.T) {
		currentSet, _ := basetypes.NewSetValueFrom(ctx, types.StringType, []string{"a"})
		targetSet := types.SetNull(nil)
		var diags diag.Diagnostics

		diff := diffStringSets(currentSet, targetSet, &diags)

		assert.False(t, diags.HasError())
		var resultSlice []string
		diff.ElementsAs(ctx, &resultSlice, false)
		assert.ElementsMatch(t, []string{"a"}, resultSlice)
	})
}
