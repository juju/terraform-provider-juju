// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/juju/errors"
	"github.com/juju/juju/api/client/cloud"
	k8s "github.com/juju/juju/caas/kubernetes"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	"k8s.io/client-go/tools/clientcmd"
)

type kubernetesCloudsClient struct {
	SharedClient
}

type CreateKubernetesCloudInput struct {
	Name                  string
	KubernetesContextName string
	KubernetesConfig      string
	ParentCloudName       string
	ParentCloudRegion     string
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
func (c *kubernetesCloudsClient) CreateKubernetesCloud(input *CreateKubernetesCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := cloud.NewClient(conn)

	conf, err := clientcmd.NewClientConfigFromBytes([]byte(input.KubernetesConfig))
	if err != nil {
		return errors.Annotate(err, "parsing kubernetes configuration data")
	}

	apiConf, err := conf.RawConfig()
	if err != nil {
		return errors.Annotate(err, "fetching kubernetes configuration")
	}

	var k8sContextName string
	if input.KubernetesContextName == "" {
		k8sContextName = apiConf.CurrentContext
	} else {
		k8sContextName = input.KubernetesContextName
	}

	newCloud, err := k8scloud.CloudFromKubeConfigContext(
		k8sContextName,
		&apiConf,
		k8scloud.CloudParamaters{
			Name:            input.Name,
			HostCloudRegion: k8s.K8sCloudOther,
		},
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = client.AddCloud(newCloud, false)
	if err != nil {
		return errors.Annotate(err, "adding kubernetes cloud")
	}

	return nil
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
