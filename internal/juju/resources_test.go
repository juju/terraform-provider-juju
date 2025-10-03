// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCharmResource_String(t *testing.T) {
	cr := CharmResource{RevisionNumber: "5", OCIImageURL: "oci-url"}
	assert.Equal(t, "5", cr.String(), "String() should return RevisionNumber if set")

	cr = CharmResource{RevisionNumber: "", OCIImageURL: "oci-url"}
	assert.Equal(t, "oci-url", cr.String(), "String() should return OCIImageURL if RevisionNumber is empty")
}

func TestCharmResources_Equal(t *testing.T) {
	empty := CharmResources{}
	nonEmpty := CharmResources{"a": CharmResource{"1", "url", "user", "pass"}}
	tests := []struct {
		name string
		a    CharmResources
		b    CharmResources
		want bool
	}{
		{"both nil", nil, nil, true},
		{"nil vs empty", nil, empty, true},
		{"empty vs empty non-nil", empty, CharmResources{}, true},
		{"same single key/value", nonEmpty, nonEmpty, true},
		{"different value for same key", nonEmpty, CharmResources{"a": CharmResource{"2", "url", "user", "pass"}}, false},
		{"missing key in other", nonEmpty, CharmResources{"b": CharmResource{"1", "url", "user", "pass"}}, false},
		{"other has extra key", nonEmpty, CharmResources{"a": CharmResource{"1", "url", "user", "pass"}, "b": CharmResource{"1", "url", "user", "pass"}}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.a.Equal(tc.b); got != tc.want {
				t.Fatalf("case %q: expected %v, got %v (a=%+v, b=%+v)", tc.name, tc.want, got, tc.a, tc.b)
			}
		})
	}
}
