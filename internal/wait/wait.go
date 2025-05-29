// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package wait

import (
	"context"
	"time"

	"github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/retry"

	"github.com/juju/terraform-provider-juju/internal/juju"
)

// Logger is an interface for logging debug messages.
type Logger interface {
	Debugf(msg string, additionalFields ...map[string]interface{})
}

// RetryConf is a struct to configure the retry behavior.
type RetryConf struct {
	// MaxDuration is the maximum duration to wait for the condition to be met.
	MaxDuration time.Duration
	// Delay is the delay between retries.
	Delay time.Duration
	// Clock is the clock to use for timing.
	Clock clock.Clock
}

// WaitForCfg is a configuration structure for the WaitFor function.
type WaitForCfg[I any, D any] struct {
	Context context.Context

	// GetData is a function that retrieves data based on the input.
	GetData GetData[I, D]
	// Input is the input to be passed to the GetData function.
	Input I
	// DataAssertions is a list of assertions to check the data against.
	// If any assertion fails, the function will return an error.
	DataAssertions []Assert[D]
	// NonFatalErrors is a list of non-fatal errors to ignore.
	NonFatalErrors []error

	// RetryConf is a configuration for retrying the operation.
	// If not provided, default values will be used.
	RetryConf *RetryConf
}

// WaitForErrorCfg is a configuration structure for the WaitForError function.
type WaitForErrorCfg[I any, D any] struct {
	Context context.Context

	// GetData is a function that retrieves data based on the input.
	GetData GetData[I, D]
	// Input is the input to be passed to the GetData function.
	Input I
	// ErrorToWait is the error to wait for.
	ErrorToWait error
	// NonFatalErrors is a list of non-fatal errors to ignore.
	NonFatalErrors []error

	// RetryConf is a configuration for retrying the operation.
	// If not provided, default values will be used.
	RetryConf *RetryConf
}

// GetData is a function type that retrieves data based on the input.
type GetData[I any, D any] func(I) (D, error)

// Assert is a function type that takes data and returns an error if the assertion fails.
type Assert[D any] func(D) error

// WaitFor waits for a condition to be met, retrying every second, by default, until the condition is met or the context is cancelled.
// It takes a function that retrieves data, an input to pass to that function, a list of assertions to check the data against,
// and a list of non-fatal errors to ignore.
func WaitFor[I any, D any](waitCfg WaitForCfg[I, D]) (D, error) {
	// Set default values for the wait configuration
	if waitCfg.RetryConf == nil {
		waitCfg.RetryConf = &RetryConf{
			MaxDuration: 30 * time.Minute,
			Delay:       time.Second,
			Clock:       clock.WallClock,
		}
	}

	var data D
	retryErr := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			data, err = waitCfg.GetData(waitCfg.Input)
			if err != nil {
				return err
			}
			for _, assert := range waitCfg.DataAssertions {
				err := assert(data)
				if err != nil {
					return err
				}
			}
			return nil
		},
		IsFatalError: func(err error) bool {
			for _, nonFatalError := range waitCfg.NonFatalErrors {
				if errors.Is(err, nonFatalError) {
					return false
				}
			}
			return true
		},
		BackoffFunc: retry.DoubleDelay,
		MaxDuration: waitCfg.RetryConf.MaxDuration,
		Delay:       waitCfg.RetryConf.Delay,
		Clock:       waitCfg.RetryConf.Clock,
		MaxDelay:    time.Minute,
		Stop:        waitCfg.Context.Done(),
	})
	return data, retryErr
}

// WaitForError waits for a specific error to be returned from the getData function.
func WaitForError[I any, D any](cfg WaitForErrorCfg[I, D]) error {
	// Set default values for the retry configuration.
	// The default max duration is 15 minutes, because this is used
	// to wait for deletion and it should take less time.
	if cfg.RetryConf == nil {
		cfg.RetryConf = &RetryConf{
			MaxDuration: 15 * time.Minute,
			Delay:       time.Second,
			Clock:       clock.WallClock,
		}
	}

	retryErr := retry.Call(retry.CallArgs{
		Func: func() error {
			_, err := cfg.GetData(cfg.Input)
			if err == nil {
				return juju.NewRetryReadError("no error returned")
			}
			if errors.Is(err, cfg.ErrorToWait) {
				return nil
			}
			return err
		},
		IsFatalError: func(err error) bool {
			for _, nonFatalError := range cfg.NonFatalErrors {
				if errors.Is(err, nonFatalError) {
					return false
				}
			}
			return true
		},
		MaxDuration: cfg.RetryConf.MaxDuration,
		Delay:       cfg.RetryConf.Delay,
		Clock:       cfg.RetryConf.Clock,
		MaxDelay:    time.Minute,
		Stop:        cfg.Context.Done(),
	})
	return retryErr
}
