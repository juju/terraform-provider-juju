// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/juju/juju/api/client/secretbackends"
)

var (
	// ErrSecretBackendNotFound is returned when a secret backend does not exist.
	ErrSecretBackendNotFound = errors.New("secret backend not found")
)

type secretBackendsClient struct {
	SharedClient
}

// CreateSecretBackendInput is the input to CreateSecretBackend.
type CreateSecretBackendInput struct {
	// Name is the name of the secret backend.
	Name string
	// BackendType is the type of the secret backend (e.g. "vault", "kubernetes").
	BackendType string
	// TokenRotateInterval is the interval at which the backend's access
	// credential/token should be rotated. Optional.
	TokenRotateInterval *time.Duration
	// Config is the backend specific configuration.
	Config map[string]any
}

// UpdateSecretBackendInput is the input to UpdateSecretBackend.
type UpdateSecretBackendInput struct {
	// Name is the name of the secret backend to update.
	Name string
	// NameChange is the new name for the secret backend. Optional.
	NameChange *string
	// TokenRotateInterval is the interval at which the backend's access
	// credential/token should be rotated. Optional.
	TokenRotateInterval *time.Duration
	// Config is the backend specific configuration to update.
	Config map[string]any
}

// RemoveSecretBackendInput is the input to RemoveSecretBackend.
type RemoveSecretBackendInput struct {
	// Name is the name of the secret backend to remove.
	Name string
}

// GetSecretBackendInput is the input to GetSecretBackend.
type GetSecretBackendInput struct {
	// Name is the name of the secret backend to read.
	Name string
}

// GetSecretBackendResponse is the response from GetSecretBackend.
type GetSecretBackendResponse struct {
	Backend secretbackends.SecretBackend
}

// ListSecretBackendsInput is the input to ListSecretBackends.
type ListSecretBackendsInput struct {
	// Names filters the list to the specified backend names. If empty, all backends are returned.
	Names []string
	// Reveal reveals secret config values.
	Reveal bool
}

// ListSecretBackendsOutput is an entry from ListSecretBackends.
type ListSecretBackendsOutput struct {
	Backend secretbackends.SecretBackend
}

func newSecretBackendsClient(sc SharedClient) *secretBackendsClient {
	return &secretBackendsClient{
		SharedClient: sc,
	}
}

// CreateSecretBackend creates a secret backend with the specified parameters.
func (c *secretBackendsClient) CreateSecretBackend(ctx context.Context, input CreateSecretBackendInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := secretbackends.NewClient(conn)

	return client.AddSecretBackend(ctx, secretbackends.CreateSecretBackend{
		Name:                input.Name,
		BackendType:         input.BackendType,
		TokenRotateInterval: input.TokenRotateInterval,
		Config:              input.Config,
	})
}

// UpdateSecretBackend updates a secret backend with the specified parameters.
func (c *secretBackendsClient) UpdateSecretBackend(ctx context.Context, input UpdateSecretBackendInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := secretbackends.NewClient(conn)

	return client.UpdateSecretBackend(ctx, secretbackends.UpdateSecretBackend{
		Name:                input.Name,
		NameChange:          input.NameChange,
		TokenRotateInterval: input.TokenRotateInterval,
		Config:              input.Config,
	}, false)
}

// RemoveSecretBackend removes the named secret backend.
func (c *secretBackendsClient) RemoveSecretBackend(ctx context.Context, input RemoveSecretBackendInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := secretbackends.NewClient(conn)

	return client.RemoveSecretBackend(ctx, input.Name, false)
}

// GetSecretBackend gets a secret backend by name.
func (c *secretBackendsClient) GetSecretBackend(ctx context.Context, input GetSecretBackendInput) (GetSecretBackendResponse, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return GetSecretBackendResponse{}, err
	}
	defer func() { _ = conn.Close() }()

	client := secretbackends.NewClient(conn)

	backends, err := client.ListSecretBackends(ctx, []string{input.Name}, false)
	if err != nil {
		return GetSecretBackendResponse{}, err
	}
	if len(backends) == 0 {
		return GetSecretBackendResponse{}, ErrSecretBackendNotFound
	}
	if len(backends) > 1 {
		return GetSecretBackendResponse{}, fmt.Errorf("expected 1 secret backend, got %d", len(backends))
	}

	// ListSecretBackends populates a per-item Error field on each result. A
	// controller reporting an item-level error would otherwise be read as a
	// successful, zero-valued backend, so surface it here. A not-found error
	// is normalized to ErrSecretBackendNotFound so callers can handle it.
	if backendErr := backends[0].Error; backendErr != nil {
		if errors.Is(backendErr, ErrSecretBackendNotFound) || strings.Contains(strings.ToLower(backendErr.Error()), "not found") {
			return GetSecretBackendResponse{}, ErrSecretBackendNotFound
		}
		return GetSecretBackendResponse{}, backendErr
	}

	return GetSecretBackendResponse{Backend: backends[0]}, nil
}

// ListSecretBackends lists secret backends, optionally filtered by name.
func (c *secretBackendsClient) ListSecretBackends(ctx context.Context, input ListSecretBackendsInput) ([]ListSecretBackendsOutput, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := secretbackends.NewClient(conn)

	backends, err := client.ListSecretBackends(ctx, input.Names, input.Reveal)
	if err != nil {
		return nil, err
	}

	result := make([]ListSecretBackendsOutput, 0, len(backends))
	for _, backend := range backends {
		result = append(result, ListSecretBackendsOutput{Backend: backend})
	}

	return result, nil
}
