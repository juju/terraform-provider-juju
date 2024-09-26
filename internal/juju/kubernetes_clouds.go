// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

type kubernetesCloudsClient struct {
	SharedClient
}

type CreateKubernetesCloudInput struct {
}

type CreateKubernetesCloudOutput struct {
}

type ReadKubernetesCloudInput struct {
}

type ReadKubernetesCloudOutput struct {
}

type UpdateKubernetesCloudInput struct {
}

type DestroyKubernetesCloudInput struct {
}

func newKubernetesCloudsClient(sc SharedClient) *kubernetesCloudsClient {
	return &kubernetesCloudsClient{
		SharedClient: sc,
	}
}

// CreateKubernetesCloud creates a new Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) CreateKubernetesCloud(input *CreateKubernetesCloudInput) (*CreateKubernetesCloudOutput, error) {
	return nil, nil
}

// ReadKubernetesCloud reads a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) ReadKubernetesCloud(input *ReadKubernetesCloudInput) (*ReadKubernetesCloudOutput, error) {
	return nil, nil
}

// UpdateKubernetesCloud updates a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) UpdateKubernetesCloud(input *UpdateKubernetesCloudInput) error {
	return nil
}

// DestroyKubernetesCloud destroys a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) DestroyKubernetesCloud(input *DestroyKubernetesCloudInput) error {
	return nil
}
