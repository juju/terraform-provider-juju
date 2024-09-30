// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/juju/errors"
	"github.com/juju/juju/api/client/cloud"
	k8s "github.com/juju/juju/caas/kubernetes"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	"k8s.io/client-go/tools/clientcmd"
	"strings"
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
	Name             string
	KubernetesConfig string
}

type ReadKubernetesCloudOutput struct {
	Name              string
	KubernetesConfig  string
	ParentCloudName   string
	ParentCloudRegion string
}

type DestroyKubernetesCloudInput struct {
	Name string
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
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := cloud.NewClient(conn)

	clouds, err := client.Clouds()
	if err != nil {
		return nil, errors.Annotate(err, "getting clouds")
	}

	for _, cloud := range clouds {
		if cloud.Name == input.Name {
			parentCloudName, parentCloudRegion := getParentCloudNameAndRegionFromHostCloudRegion(cloud.HostCloudRegion)
			return &ReadKubernetesCloudOutput{
				Name:              input.Name,
				ParentCloudName:   parentCloudName,
				ParentCloudRegion: parentCloudRegion,
				KubernetesConfig:  input.KubernetesConfig,
			}, nil
		}
	}

	return nil, errors.NotFoundf("kubernetes cloud %q", input.Name)
}

// getParentCloudNameAndRegionFromHostCloudRegion returns the parent cloud name and region from the host cloud region.
// HostCloudRegion represents the k8s host cloud. The format is <cloudName>/<region>.
func getParentCloudNameAndRegionFromHostCloudRegion(hostCloudRegion string) (string, string) {
	parts := strings.Split(hostCloudRegion, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// RemoveKubernetesCloud removes a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) RemoveKubernetesCloud(input *DestroyKubernetesCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := cloud.NewClient(conn)

	err = client.RemoveCloud(input.Name)
	if err != nil {
		return errors.Annotate(err, "removing kubernetes cloud")
	}

	return nil
}
