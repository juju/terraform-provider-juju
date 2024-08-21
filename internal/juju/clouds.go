// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

type cloudsClient struct {
	SharedClient
}

type CreateCloudInput struct {
}

type CreateCloudOutput struct {
}

type ReadCloudInput struct {
}

type ReadCloudOutput struct {
}

type UpdateCloudInput struct {
}

type DestroyCloudInput struct {
}

func newCloudsClient(sc SharedClient) *cloudsClient {
	return &cloudsClient{
		SharedClient: sc,
	}
}

func (c *cloudsClient) CreateCloud(input *CreateCloudInput) (*CreateCloudOutput, error) {
	return nil, nil
}

func (c *cloudsClient) ReadCloud(input *ReadCloudInput) (*ReadCloudOutput, error) {
	return nil, nil
}

func (c *cloudsClient) UpdateCloud(input *UpdateCloudInput) error {
	return nil
}

func (c *cloudsClient) DestroyCloud(input *DestroyCloudInput) error {
	return nil
}
