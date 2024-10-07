// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	k8s "github.com/juju/juju/caas/kubernetes"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesCloudSuite struct {
	suite.Suite
	JujuSuite

	mockKubernetesCloudClient *MockKubernetesCloudAPIClient
}

func (s *KubernetesCloudSuite) SetupSuite() {
	s.testModelName = strPtr("test-kubernetes-cloud-model")
}

func (s *KubernetesCloudSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := s.JujuSuite.setupMocks(t)
	s.mockKubernetesCloudClient = NewMockKubernetesCloudAPIClient(ctlr)

	return ctlr
}

func getFakeCloudConfig() string {
	return `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ZmFrZS1jZXJ0aWZpY2F0ZS1hdXRob3JpdHktZGF0YQ==
    server: https://10.172.195.202:16443
  name: microk8s-cluster
contexts:
- context:
    cluster: microk8s-cluster
    user: admin
  name: fake-cloud-context
current-context: fake-cloud-context
kind: Config
preferences: {}
users:
- name: admin
  user:
    client-certificate-data: ZmFrZS1jbGllbnQtY2VydGlmaWNhdGUtZGF0YQ==
    client-key-data: ZmFrZS1jbGllbnQta2V5LWRhdGE=
`
}

func (s *KubernetesCloudSuite) TestCreateKubernetesCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	s.mockKubernetesCloudClient.EXPECT().AddCloud(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	fakeCloudConfig, err := clientcmd.NewClientConfigFromBytes([]byte(getFakeCloudConfig()))
	s.Require().NoError(err)

	fakeApiConfig, err := fakeCloudConfig.RawConfig()
	s.Require().NoError(err)

	fakeCloud, err := k8scloud.CloudFromKubeConfigContext(
		"fake-cloud-context",
		&fakeApiConfig,
		k8scloud.CloudParamaters{
			Name:            "fake-cloud",
			HostCloudRegion: k8s.K8sCloudOther,
		},
	)
	s.Require().NoError(err)

	err = s.mockKubernetesCloudClient.AddCloud(fakeCloud, false)
	s.Require().NoError(err)
}

func (s *KubernetesCloudSuite) TestUpdateKubernetesCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	s.mockKubernetesCloudClient.EXPECT().UpdateCloud(gomock.Any()).Return(nil).AnyTimes()

	fakeCloudConfig, err := clientcmd.NewClientConfigFromBytes([]byte(getFakeCloudConfig()))
	s.Require().NoError(err)

	fakeApiConfig, err := fakeCloudConfig.RawConfig()
	s.Require().NoError(err)

	fakeCloud, err := k8scloud.CloudFromKubeConfigContext(
		"fake-cloud-context",
		&fakeApiConfig,
		k8scloud.CloudParamaters{
			Name:            "fake-cloud",
			HostCloudRegion: k8s.K8sCloudOther,
		},
	)
	s.Require().NoError(err)

	err = s.mockKubernetesCloudClient.UpdateCloud(fakeCloud)
	s.Require().NoError(err)
}

func (s *KubernetesCloudSuite) TestRemoveKubernetesCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	s.mockKubernetesCloudClient.EXPECT().RemoveCloud(gomock.Any()).Return(nil).AnyTimes()

	err := s.mockKubernetesCloudClient.RemoveCloud("fake-cloud")
	s.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestKubernetesCloudSuite(t *testing.T) {
	suite.Run(t, new(KubernetesCloudSuite))
}
