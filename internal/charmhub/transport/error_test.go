// Copyright 2020 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package transport

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoErrors(t *testing.T) {
	var errors APIErrors
	err := errors.Error()
	require.Equal(t, err, "")
}

func TestNoErrorsWithEmptySlice(t *testing.T) {
	errors := make(APIErrors, 0)
	err := errors.Error()
	require.Equal(t, err, "")
}

func TestWithOneError(t *testing.T) {
	errors := APIErrors{{
		Message: "one",
	}}
	err := errors.Error()
	require.Equal(t, err, `one`)
}

func TestWithMultipleErrors(t *testing.T) {
	errors := APIErrors{
		{Message: "one"},
		{Message: "two"},
	}
	err := errors.Error()
	require.Equal(t, err, `one
two`)
}

func TestExtras(t *testing.T) {
	expected := APIError{
		Extra: APIErrorExtra{
			DefaultBases: []Base{
				{Architecture: "amd64", Name: "ubuntu", Channel: "20.04"},
			},
		},
	}
	bytes, err := json.Marshal(expected)
	require.Nil(t, err)

	var result APIError
	err = json.Unmarshal(bytes, &result)
	require.Nil(t, err)

	require.Equal(t, result, expected)
}
