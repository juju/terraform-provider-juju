// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package charmhub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const refreshSuccessBody = `{
  "results": [{
    "charm": {
      "revision": 42,
      "bases": [{"name": "ubuntu", "channel": "22.04", "architecture": "amd64"}]
    },
    "effective-channel": "latest/stable",
    "name": "testcharm",
    "result": "install",
    "instance-key": "key-0"
  }]
}`

// newTestClient returns a client pointed at url with retry delays short
// enough for tests.
func newTestClient(url string) *Client {
	c := New(url, nil)
	c.retryDelay = time.Millisecond
	c.retryMaxDelay = 2 * time.Millisecond
	return c
}

func TestRefreshRetriesOnServerError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(refreshSuccessBody))
	}))
	defer srv.Close()

	result, err := newTestClient(srv.URL).Refresh(context.Background(), CharmRefreshInput{
		Name:    "testcharm",
		Base:    "ubuntu@22.04",
		Channel: "latest/stable",
	})
	require.NoError(t, err)
	require.Equal(t, int32(3), calls.Load())
	require.Equal(t, 42, result.Revision)
	require.Equal(t, "latest/stable", result.Channel)
	require.Equal(t, "ubuntu@22.04", result.Base)
}

func TestRefreshRetriesOnDroppedConnection(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			// Drop the connection without a response, so the
			// client sees EOF, as observed with api.charmhub.io.
			conn, _, err := w.(http.Hijacker).Hijack()
			require.NoError(t, err)
			conn.Close()
			return
		}
		_, _ = w.Write([]byte(refreshSuccessBody))
	}))
	defer srv.Close()

	result, err := newTestClient(srv.URL).Refresh(context.Background(), CharmRefreshInput{
		Name:    "testcharm",
		Base:    "ubuntu@22.04",
		Channel: "latest/stable",
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), calls.Load())
	require.Equal(t, 42, result.Revision)
}

func TestRefreshDoesNotRetryOnClientError(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).Refresh(context.Background(), CharmRefreshInput{
		Name:    "testcharm",
		Base:    "ubuntu@22.04",
		Channel: "latest/stable",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "status 404")
	require.Equal(t, int32(1), calls.Load())
}

func TestRefreshGivesUpAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).Refresh(context.Background(), CharmRefreshInput{
		Name:    "testcharm",
		Base:    "ubuntu@22.04",
		Channel: "latest/stable",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "status 503")
	require.Equal(t, int32(defaultRetryAttempts), calls.Load())
}

func TestRefreshStopsRetryingOnContextCancel(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL, nil)
	c.retryDelay = time.Hour // the cancelled context must interrupt the delay
	c.retryMaxDelay = time.Hour

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := c.Refresh(ctx, CharmRefreshInput{
			Name:    "testcharm",
			Base:    "ubuntu@22.04",
			Channel: "latest/stable",
		})
		done <- err
	}()

	// Wait for the first attempt to fail, then cancel while the client
	// is waiting to retry.
	require.Eventually(t, func() bool { return calls.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Refresh did not return after context cancellation")
	}
	require.Equal(t, int32(1), calls.Load())
}
