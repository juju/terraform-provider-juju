// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package wait_test

import (
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
	logCounter := atomic.Int32{}
	testFunc := func(string) (string, error) {
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
		Logf: func(msg string, additionalFields ...map[string]interface{}) {
			if msg != "waiting for condition" {
				t.Fatalf("expected retry log message, got %q", msg)
			}
			if len(additionalFields) != 1 {
				t.Fatalf("expected one map of additional fields, got %d", len(additionalFields))
			}
			if _, ok := additionalFields[0]["attempt"].(int); !ok {
				t.Fatalf("expected attempt field to be an int, got %T", additionalFields[0]["attempt"])
			}
			if got := additionalFields[0]["last_error"]; !errors.Is(got.(error), juju.RetryReadError) {
				t.Fatalf("expected last_error field to match retry error, got %v", got)
			}
			logCounter.Add(1)
		},
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if counter.Load() != 11 {
		t.Fatalf("expected 11 calls, got %d", counter.Load())
	}
	if result != "success" {
		t.Fatalf("expected success, got %v", result)
	}
	if logCounter.Load() != 11 {
		t.Fatalf("expected 11 log calls, got %d", logCounter.Load())
	}
}

func TestWaitForFatalError(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	fatalError := errors.New("fatal error")
	counter := atomic.Int32{}
	testFunc := func(string) (string, error) {
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
	testFunc := func(string) (string, error) {
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
	logCounter := atomic.Int32{}
	testFunc := func(string) (string, error) {
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
		Logf: func(msg string, additionalFields ...map[string]interface{}) {
			if msg != "waiting for expected error" {
				t.Fatalf("expected retry log message, got %q", msg)
			}
			if len(additionalFields) != 1 {
				t.Fatalf("expected one map of additional fields, got %d", len(additionalFields))
			}
			if got := additionalFields[0]["expected_error"]; got != juju.ApplicationNotFoundError {
				t.Fatalf("expected expected_error field to match, got %v", got)
			}
			if _, ok := additionalFields[0]["attempt"].(int); !ok {
				t.Fatalf("expected attempt field to be an int, got %T", additionalFields[0]["attempt"])
			}
			if got := additionalFields[0]["last_error"]; !errors.Is(got.(error), juju.RetryReadError) {
				t.Fatalf("expected last_error field to match retry error, got %v", got)
			}
			logCounter.Add(1)
		},
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
	if logCounter.Load() != 10 {
		t.Fatalf("expected 10 log calls, got %d", logCounter.Load())
	}
}

func TestWaitForError_RetryAllErrors(t *testing.T) {
	autoAdvancingClock := createAutoAdvancingClock(time.Now())
	counter := atomic.Int32{}
	testFunc := func(string) (string, error) {
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
	testFunc := func(string) (string, error) {
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
