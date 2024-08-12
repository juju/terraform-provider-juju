// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"testing"

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
