// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package wait_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/juju/clock/testclock"

	"github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

func TestWaitFor(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	counter := atomic.Int32{}
	testFunc := func(context.Context, string) (string, error) {
		if counter.Load() < 10 {
			counter.Add(1)
			return "", juju.RetryReadError
		}
		if counter.Load() == 10 {
			counter.Add(1)
			return "wrong_string", nil
		}
		return "success", nil
	}

	result, err := wait.WaitFor(wait.WaitForCfg[string, string]{
		Context: t.Context(),
		GetData: testFunc,
		Input:   "test",
		DataAssertions: []wait.Assert[string]{
			func(s1 string) error {
				if s1 != "success" {
					return juju.RetryReadError
				}
				return nil
			},
		},
		NonFatalErrors: []error{juju.RetryReadError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 60 * time.Second,
			Delay:       1 * time.Second,
			Clock:       autoAdvancingClock,
			MaxDelay:    time.Second,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if counter.Load() != 11 {
		t.Fatalf("expected 10 calls, got %d", counter.Load())
	}
	if result != "success" {
		t.Fatalf("expected success, got %v", result)
	}
}

func TestWaitForFatalError(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	fatalError := errors.New("fatal error")
	counter := atomic.Int32{}
	testFunc := func(context.Context, string) (string, error) {
		counter.Add(1)
		return "", fatalError
	}
	_, err := wait.WaitFor(wait.WaitForCfg[string, string]{
		Context:        t.Context(),
		GetData:        testFunc,
		Input:          "test",
		DataAssertions: []wait.Assert[string]{},
		NonFatalErrors: []error{juju.RetryReadError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 60 * time.Second,
			Delay:       1 * time.Second,
			Clock:       autoAdvancingClock,
			MaxDelay:    time.Second,
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, fatalError) {
		t.Fatalf("expected different error, got %v", err)
	}
	if counter.Load() != 1 {
		t.Fatalf("expected 1 call, got %d", counter.Load())
	}
}

func TestWaitForMaxDuration(t *testing.T) {
	now := time.Now()
	autoAdvancingClock := createAutoAdvancingClock(now)
	testFunc := func(context.Context, string) (string, error) {
		return "", juju.RetryReadError
	}
	_, err := wait.WaitFor(wait.WaitForCfg[string, string]{
		Context:        t.Context(),
		GetData:        testFunc,
		Input:          "test",
		DataAssertions: []wait.Assert[string]{},
		NonFatalErrors: []error{juju.RetryReadError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 1 * time.Second,
			Delay:       1 * time.Second,
			Clock:       autoAdvancingClock,
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, juju.RetryReadError) {
		t.Fatalf("expected different error, got %v", err)
	}
	if autoAdvancingClock.Now().Sub(now) != 1*time.Second {
		t.Fatalf("expected clock to advance at least 1 second, got %v", autoAdvancingClock.Now().Sub(now))
	}
}

func TestWaitForError(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	counter := atomic.Int32{}
	testFunc := func(context.Context, string) (string, error) {
		if counter.Load() < 10 {
			counter.Add(1)
			return "", juju.RetryReadError
		}
		return "", juju.ApplicationNotFoundError
	}
	err := wait.WaitForError(wait.WaitForErrorCfg[string, string]{
		Context:        t.Context(),
		GetData:        testFunc,
		Input:          "test",
		ExpectedErr:    juju.ApplicationNotFoundError,
		NonFatalErrors: []error{juju.RetryReadError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 60 * time.Second,
			Delay:       1 * time.Second,
			Clock:       autoAdvancingClock,
			MaxDelay:    time.Second,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if counter.Load() != 10 {
		t.Fatalf("expected 10 calls, got %d", counter.Load())
	}
}

func TestWaitForError_RetryAllErrors(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	counter := atomic.Int32{}
	testFunc := func(context.Context, string) (string, error) {
		switch counter.Load() {
		case 0:
			counter.Add(1)
			return "", juju.RetryReadError
		case 1:
			counter.Add(1)
			return "", errors.New("some other error")
		case 2:
			counter.Add(1)
			return "", errors.New("yet another error")
		}
		return "", juju.ApplicationNotFoundError
	}
	err := wait.WaitForError(wait.WaitForErrorCfg[string, string]{
		Context:        t.Context(),
		GetData:        testFunc,
		Input:          "test",
		ExpectedErr:    juju.ApplicationNotFoundError,
		RetryAllErrors: true,
		RetryConf: &wait.RetryConf{
			MaxDuration: 60 * time.Second,
			Delay:       1 * time.Second,
			Clock:       autoAdvancingClock,
			MaxDelay:    time.Second,
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if counter.Load() != 3 {
		t.Fatalf("expected 3 calls, got %d", counter.Load())
	}
}

func TestWaitForError_MaxDuration(t *testing.T) {
	now := time.Now()
	autoAdvancingClock := createAutoAdvancingClock(now)
	testFunc := func(context.Context, string) (string, error) {
		return "", juju.RetryReadError
	}
	err := wait.WaitForError(wait.WaitForErrorCfg[string, string]{
		Context:        t.Context(),
		GetData:        testFunc,
		Input:          "test",
		ExpectedErr:    juju.ApplicationNotFoundError,
		NonFatalErrors: []error{juju.RetryReadError},
		RetryConf: &wait.RetryConf{
			MaxDuration: 1 * time.Second,
			Delay:       1 * time.Second,
			MaxDelay:    time.Second,
			Clock:       autoAdvancingClock,
		},
	})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, juju.RetryReadError) {
		t.Fatalf("expected different error, got %v", err)
	}
	if autoAdvancingClock.Now().Sub(now) != 1*time.Second {
		t.Fatalf("expected clock to advance at least 1 second, got %v", autoAdvancingClock.Now().Sub(now))
	}
}

func createAutoAdvancingClock(now time.Time) *testclock.AutoAdvancingClock {
	testClock := testclock.NewClock(now)
	return &testclock.AutoAdvancingClock{
		Clock: testClock,
		Advance: func(d time.Duration) {
			testClock.Advance(d)
		},
	}
}
