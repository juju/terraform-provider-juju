// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidIPAddressCondition(t *testing.T) {
	valid := []string{"any", "public", "private", "10.0.10.0/24", "2001:db8::/32", "0.0.0.0/0"}
	for _, cond := range valid {
		assert.True(t, ValidIPAddressCondition(cond), "expected %q to be valid", cond)
	}

	invalid := []string{"", "foo", "10.0.0.1", "10.0.0.0/33", "PUBLIC", "Any", "10.0.0.0 /24"}
	for _, cond := range invalid {
		assert.False(t, ValidIPAddressCondition(cond), "expected %q to be invalid", cond)
	}
}

func TestMatchIPAddresses(t *testing.T) {
	type testCase struct {
		name       string
		all        []string
		conditions []string
		expected   []string
		expectErr  error // nil, ErrNoMatchingIPAddress (retriable), or any other (fatal)
	}

	testCases := []testCase{
		{
			name:       "any matches first available ip",
			all:        []string{"10.0.0.1", "1.1.1.1"},
			conditions: []string{"any"},
			expected:   []string{"10.0.0.1"},
		},
		{
			name:       "public matches non-private ip",
			all:        []string{"10.0.0.1", "192.168.1.1", "1.1.1.1"},
			conditions: []string{"public"},
			expected:   []string{"1.1.1.1"},
		},
		{
			name:       "private matches private ip",
			all:        []string{"1.1.1.1", "192.168.1.5"},
			conditions: []string{"private"},
			expected:   []string{"192.168.1.5"},
		},
		{
			name:       "cidr matches ip within range",
			all:        []string{"10.0.0.1", "10.0.10.5", "10.0.10.9"},
			conditions: []string{"10.0.10.0/24"},
			expected:   []string{"10.0.10.5"},
		},
		{
			name:       "cidr does not match ip outside range",
			all:        []string{"10.0.0.1", "192.168.1.1"},
			conditions: []string{"10.0.10.0/24"},
			expectErr:  ErrNoMatchingIPAddress,
		},
		{
			name:       "two any conditions consume distinct ips",
			all:        []string{"10.0.0.1", "10.0.0.2"},
			conditions: []string{"any", "any"},
			expected:   []string{"10.0.0.1", "10.0.0.2"},
		},
		{
			name:       "public and private match distinct ips",
			all:        []string{"10.0.0.1", "1.1.1.1"},
			conditions: []string{"public", "private"},
			expected:   []string{"1.1.1.1", "10.0.0.1"},
		},
		{
			name:       "not enough ips for conditions",
			all:        []string{"10.0.0.1"},
			conditions: []string{"any", "any"},
			expectErr:  ErrNoMatchingIPAddress,
		},
		{
			name:       "no ips reported",
			all:        []string{},
			conditions: []string{"any"},
			expectErr:  ErrNoMatchingIPAddress,
		},
		{
			name:       "unparseable ip is a fatal error",
			all:        []string{"not-an-ip", "10.0.0.7"},
			conditions: []string{"private"},
			expectErr:  errors.New("fatal"),
		},
		{
			name:       "ipv6 private matches",
			all:        []string{"fd00::1", "2001:db8::1"},
			conditions: []string{"private"},
			expected:   []string{"fd00::1"},
		},
		{
			name:       "ipv6 cidr matches",
			all:        []string{"fd00::1", "2001:db8::5"},
			conditions: []string{"2001:db8::/32"},
			expected:   []string{"2001:db8::5"},
		},
		{
			name:       "empty conditions returns empty result",
			all:        []string{"10.0.0.1"},
			conditions: []string{},
			expected:   []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched, err := MatchIPAddresses(tc.all, tc.conditions)
			if tc.expectErr != nil {
				require.Error(t, err)
				assert.Nil(t, matched)
				if tc.expectErr == ErrNoMatchingIPAddress {
					assert.True(t, errors.Is(err, ErrNoMatchingIPAddress), "expected retriable ErrNoMatchingIPAddress, got: %v", err)
				} else {
					assert.False(t, errors.Is(err, ErrNoMatchingIPAddress), "expected fatal error, got retriable: %v", err)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, matched)
		})
	}
}
