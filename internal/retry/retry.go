// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package retry

import (
	"context"

	"github.com/juju/terraform-provider-juju/internal/wait"
)

// Do is a function type to execute an operation.
type Do[I any, D any] = wait.GetData[I, D]

// Assert is a function type that takes data and returns an error if the assertion fails.
type Assert[D any] = wait.Assert[D]

// RetryConf is a struct to configure the retry behavior.
type RetryConf = wait.RetryConf

// RetryOnErrorsCfg is a configuration structure for the RetryOnErrors function.
type RetryOnErrorsCfg[I any, D any] struct {
	Context context.Context

	// GetData is a function that retrieves data based on the input.
	Do Do[I, D]
	// Input is the input to be passed to the GetData function.
	Input I
	// DataAssertions is a list of assertions to check the data against.
	// If any assertion fails, the function will return an error.
	DataAssertions []Assert[D]
	// RetriableErrors is to retry on.
	RetriableErrors []error

	// RetryConf is a configuration for retrying the operation.
	// If not provided, default values will be used.
	RetryConf *RetryConf
}

// RetryOnErrors waits for a condition to be met, retrying every second, by default, until the condition is met or the context is cancelled.
// It takes a function that retrieves data, an input to pass to that function, a list of assertions to check the data against,
// and a list of retriable errors to ignore.
func RetryOnErrors[I any, D any](retryConf RetryOnErrorsCfg[I, D]) (D, error) {
	return wait.WaitFor(
		wait.WaitForCfg[I, D]{
			Context:        retryConf.Context,
			GetData:        retryConf.Do,
			Input:          retryConf.Input,
			DataAssertions: retryConf.DataAssertions,
			NonFatalErrors: retryConf.RetriableErrors,
			RetryConf:      retryConf.RetryConf,
		})
}
