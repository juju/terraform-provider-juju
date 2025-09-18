// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/juju/charm/v12"
	"github.com/juju/juju/core/crossmodel"
	"github.com/stretchr/testify/assert"
)

func TestFilterByEndpoints(t *testing.T) {
	testCases := []struct {
		name           string
		endpoints      []string
		offers         []*crossmodel.ApplicationOfferDetails
		expectedOffers []*crossmodel.ApplicationOfferDetails
	}{
		{
			name:      "no endpoints specified returns all offers",
			endpoints: nil,
			offers: []*crossmodel.ApplicationOfferDetails{
				{OfferName: "a"},
				{OfferName: "b"},
				{OfferName: "c"},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{
				{OfferName: "a"},
				{OfferName: "b"},
				{OfferName: "c"},
			},
		},
		{
			name:      "filter by single endpoint",
			endpoints: []string{"db"},
			offers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
					},
				},
				{
					OfferName: "b",
					Endpoints: []charm.Relation{
						{Name: "cache"},
					},
				},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
					},
				},
			},
		},
		{
			name:      "filter by multiple endpoints",
			endpoints: []string{"db", "cache"},
			offers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
						{Name: "cache"},
					},
				},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
						{Name: "cache"},
					},
				},
			},
		},
		{
			name:      "no matching endpoints returns no offers",
			endpoints: []string{"db"},
			offers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "cache"},
					},
				},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{},
		},
		{
			name:      "partial match returns no offers",
			endpoints: []string{"db", "cache"},
			offers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
					},
				},
				{
					OfferName: "b",
					Endpoints: []charm.Relation{
						{Name: "cache"},
					},
				},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{},
		},
		{
			name:      "extra endpoints in offer returns no offers",
			endpoints: []string{"db"},
			offers: []*crossmodel.ApplicationOfferDetails{
				{
					OfferName: "a",
					Endpoints: []charm.Relation{
						{Name: "db"},
						{Name: "cache"},
					},
				},
			},
			expectedOffers: []*crossmodel.ApplicationOfferDetails{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			filtered := filterByEndpoints(tc.offers, tc.endpoints)
			assert.Equal(t, tc.expectedOffers, filtered, "expected offers %v, got %v", tc.expectedOffers, filtered)
		})
	}
}
