// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"fmt"
	"testing"

	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// testAccCheckJaasResourceAccess verifies that no direct relations exist
// between the object and target.
// Object and target are expected to be Juju tags of the form <resource-type>:<id>
// Use newCheckAttribute to fetch and format resource tags from computed resources.
func testAccCheckJaasResourceAccess(relation string, object, target *string, expectedAccess bool) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		if object == nil {
			return fmt.Errorf("no object set")
		}
		if target == nil {
			return fmt.Errorf("no target set")
		}
		conn, err := TestClient.Models.GetConnection(nil)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		jc := api.NewClient(conn)
		req := params.ListRelationshipTuplesRequest{
			Tuple: params.RelationshipTuple{
				Object:       *object,
				Relation:     relation,
				TargetObject: *target,
			},
		}
		resp, err := jc.ListRelationshipTuples(&req)
		if err != nil {
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
