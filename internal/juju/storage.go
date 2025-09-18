// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"errors"

	"github.com/juju/juju/api/client/storage"
	"github.com/juju/juju/rpc/params"
)

var (
	// NoSuchProviderError is returned when a storage provider does not exist.
	NoSuchProviderError = errors.New("no such provider")
)

type storageClient struct {
	SharedClient
}

func newStorageClient(sc SharedClient) *storageClient {
	return &storageClient{
		SharedClient: sc,
	}
}

// CreatePool creates pool with specified parameters.
func (c *storageClient) CreatePool(modelname, pname, provider string, attrs map[string]interface{}) error {
	conn, err := c.GetConnection(&modelname)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.CreatePool(pname, provider, attrs)
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
func (c *storageClient) RemovePool(modelname, pname string) error {
	conn, err := c.GetConnection(&modelname)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.RemovePool(pname)
}

// GetPool gets a pool by name.
func (c *storageClient) GetPool(modelname, pname string) (params.StoragePool, error) {
	conn, err := c.GetConnection(&modelname)
	if err != nil {
		return params.StoragePool{}, err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	pools, err := client.ListPools([]string{}, []string{pname})
	if err != nil {
		return params.StoragePool{}, err
	}
	if len(pools) == 0 {
		return params.StoragePool{}, NoSuchProviderError
	}

	return pools[0], nil
}
