// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"errors"

	"github.com/juju/juju/api/client/storage"
	"github.com/juju/juju/rpc/params"
)

var (
	// StoragePoolNotFoundError is returned when a storage pool does not exist.
	StoragePoolNotFoundError = errors.New("storage pool not found")
)

type storageClient struct {
	SharedClient
}

type CreateStoragePoolInput struct {
	ModelUUID string
	PoolName  string
	Provider  string
	Attrs     map[string]interface{}
}

type UpdateStoragePoolInput struct {
	ModelUUID string
	PoolName  string
	Provider  string
	Attrs     map[string]interface{}
}

type RemoveStoragePoolInput struct {
	ModelUUID string
	PoolName  string
}

type GetStoragePoolInput struct {
	ModelUUID string
	PoolName  string
}

type GetStoragePoolResponse struct {
	Pool params.StoragePool
}

func newStorageClient(sc SharedClient) *storageClient {
	return &storageClient{
		SharedClient: sc,
	}
}

// CreatePool creates pool with specified parameters.
func (c *storageClient) CreatePool(input CreateStoragePoolInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.CreatePool(input.PoolName, input.Provider, input.Attrs)
}

// UpdatePool updates a pool with specified parameters.
func (c *storageClient) UpdatePool(modelname, pname, provider string, attrs map[string]interface{}) error {
	conn, err := c.GetConnection(&modelname)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.UpdatePool(pname, provider, attrs)
}

// RemovePool removes the named pool.
func (c *storageClient) RemovePool(input RemoveStoragePoolInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.RemovePool(input.PoolName)
}

// GetPool gets a pool by name.
func (c *storageClient) GetPool(input GetStoragePoolInput) (GetStoragePoolResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return GetStoragePoolResponse{}, err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	pools, err := client.ListPools([]string{}, []string{input.PoolName})
	if err != nil {
		return GetStoragePoolResponse{}, err
	}
	if len(pools) == 0 {
		return GetStoragePoolResponse{}, StoragePoolNotFoundError
	}

	return GetStoragePoolResponse{Pool: pools[0]}, nil
}
