// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"errors"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/storage"
	"github.com/juju/juju/rpc/params"
)

var (
	// ErrStoragePoolNotFound is returned when a storage pool does not exist.
	ErrStoragePoolNotFound = errors.New("storage pool not found")
)

type storageClient struct {
	SharedClient
}

// CreateStoragePoolInput is the input to CreatePool.
type CreateStoragePoolInput struct {
	ModelUUID string
	PoolName  string
	Provider  string
	Attrs     map[string]interface{}
}

// UpdateStoragePoolInput is the input to UpdatePool.
type UpdateStoragePoolInput struct {
	ModelUUID string
	PoolName  string
	Provider  string
	Attrs     map[string]interface{}
}

// RemoveStoragePoolInput is the input to RemovePool.
type RemoveStoragePoolInput struct {
	ModelUUID string
	PoolName  string
}

// GetStoragePoolInput is the input to GetPool.
type GetStoragePoolInput struct {
	ModelUUID string
	PoolName  string
}

// GetStoragePoolResponse is the response from GetPool.
type GetStoragePoolResponse struct {
	Pool params.StoragePool
}

// ListStoragePoolsInput is the input to ListPools.
type ListStoragePoolsInput struct {
	ModelUUID string
	Providers []string
	Names     []string
}

// ListStoragePoolsOutput is an entry from ListPools.
type ListStoragePoolsOutput struct {
	Pool params.StoragePool
}

func newStorageClient(sc SharedClient) *storageClient {
	return &storageClient{
		SharedClient: sc,
	}
}

// CreatePool creates pool with specified parameters.
func (c *storageClient) CreatePool(ctx context.Context, input CreateStoragePoolInput) error {
	return withConnection(ctx, c.SharedClient, &input.ModelUUID, func(conn api.Connection) error {
		client := storage.NewClient(conn)

		return client.CreatePool(ctx, input.PoolName, input.Provider, input.Attrs)
	})
}

// UpdatePool updates a pool with specified parameters.
func (c *storageClient) UpdatePool(ctx context.Context, modeluuid, pname, provider string, attrs map[string]interface{}) error {
	return withConnection(ctx, c.SharedClient, &modeluuid, func(conn api.Connection) error {
		client := storage.NewClient(conn)

		return client.UpdatePool(ctx, pname, provider, attrs)
	})
}

// RemovePool removes the named pool.
func (c *storageClient) RemovePool(ctx context.Context, input RemoveStoragePoolInput) error {
	return withConnection(ctx, c.SharedClient, &input.ModelUUID, func(conn api.Connection) error {
		client := storage.NewClient(conn)

		return client.RemovePool(ctx, input.PoolName)
	})
}

// GetPool gets a pool by name.
func (c *storageClient) GetPool(ctx context.Context, input GetStoragePoolInput) (GetStoragePoolResponse, error) {
	var out GetStoragePoolResponse
	err := withConnection(ctx, c.SharedClient, &input.ModelUUID, func(conn api.Connection) error {
		client := storage.NewClient(conn)

		pools, err := client.ListPools(ctx, []string{}, []string{input.PoolName})
		if err != nil {
			return err
		}
		if len(pools) == 0 {
			return ErrStoragePoolNotFound
		}

		out = GetStoragePoolResponse{Pool: pools[0]}
		return nil
	})
	return out, err
}

// ListPools lists pools, optionally filtered by provider and/or name.
func (c *storageClient) ListPools(ctx context.Context, input ListStoragePoolsInput) ([]ListStoragePoolsOutput, error) {
	var out []ListStoragePoolsOutput
	err := withConnection(ctx, c.SharedClient, &input.ModelUUID, func(conn api.Connection) error {
		client := storage.NewClient(conn)

		pools, err := client.ListPools(ctx, input.Providers, input.Names)
		if err != nil {
			return err
		}

		result := make([]ListStoragePoolsOutput, 0, len(pools))
		for _, pool := range pools {
			result = append(result, ListStoragePoolsOutput{Pool: pool})
		}

		out = result
		return nil
	})
	return out, err
}
