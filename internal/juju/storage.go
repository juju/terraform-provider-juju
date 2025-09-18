// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/juju/juju/api/client/storage"
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
func (c *storageClient) CreatePool(pname, provider string, attrs map[string]interface{}) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.CreatePool(pname, provider, attrs)
}

// UpdatePool updates a pool with specified parameters.
func (c *storageClient) UpdatePool(pname, provider string, attrs map[string]interface{}) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.UpdatePool(pname, provider, attrs)
}

// RemovePool removes the named pool.
func (c *storageClient) RemovePool(pname string) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := storage.NewClient(conn)

	return client.RemovePool(pname)
}
