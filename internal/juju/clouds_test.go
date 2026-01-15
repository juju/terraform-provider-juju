// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/juju/juju/api"
	k8s "github.com/juju/juju/caas/kubernetes"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	"k8s.io/client-go/tools/clientcmd"
)

type CloudSuite struct {
	suite.Suite
	JujuSuite

	mockCloudClient  *MockCloudAPIClient
	mockSharedClient *MockSharedClient
	mockConnection   *MockConnection
}

func (s *CloudSuite) SetupSuite() {
	s.testModelName = strPtr("test-kubernetes-cloud-model")
}

func (s *CloudSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctrl := s.JujuSuite.setupMocks(t)
	s.mockCloudClient = NewMockCloudAPIClient(ctrl)
	s.mockSharedClient = NewMockSharedClient(ctrl)
	s.mockConnection = NewMockConnection(ctrl)

	return ctrl
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

func (s *CloudSuite) TestCreateKubernetesCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	s.mockCloudClient.EXPECT().AddCloud(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.mockCloudClient.EXPECT().AddCredential(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	fakeCloudConfig, err := clientcmd.NewClientConfigFromBytes([]byte(getFakeCloudConfig()))
	s.Require().NoError(err)

	fakeApiConfig, err := fakeCloudConfig.RawConfig()
	s.Require().NoError(err)

	fakeContextName := "fake-cloud-context"

	fakeCloudRegion := k8s.K8sCloudOther

	fakeCloud, err := k8scloud.CloudFromKubeConfigContext(
		fakeContextName,
		&fakeApiConfig,
		k8scloud.CloudParamaters{
			Name:            "fake-cloud",
			HostCloudRegion: fakeCloudRegion,
		},
	)
	s.Require().NoError(err)

	err = s.mockCloudClient.AddCloud(fakeCloud, false)
	s.Require().NoError(err)

	fakeCredential, err := k8scloud.CredentialFromKubeConfigContext(fakeContextName, &fakeApiConfig)
	s.Require().NoError(err)

	fakeCloudCredTag := "fake-cloud-cred"

	err = s.mockCloudClient.AddCredential(fakeCloudCredTag, fakeCredential)
	s.Require().NoError(err)
}

func (s *CloudSuite) TestUpdateKubernetesCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	s.mockCloudClient.EXPECT().UpdateCloud(gomock.Any()).Return(nil).AnyTimes()

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

	err = s.mockCloudClient.UpdateCloud(fakeCloud)
	s.Require().NoError(err)
}

func (s *CloudSuite) TestAddCloud() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	cc := &cloudsClient{
		SharedClient: s.mockSharedClient,
		getCloudAPIClient: func(connection api.Connection) CloudAPIClient {
			return s.mockCloudClient
		},
	}

	s.mockSharedClient.EXPECT().GetConnection(gomock.Any()).Return(s.mockConnection, nil).AnyTimes()
	s.mockConnection.EXPECT().Close().Return(nil).AnyTimes()

	// Expect default region to be set when Regions is omitted.
	s.mockCloudClient.EXPECT().AddCloud(gomock.Any(), false).
		DoAndReturn(func(cloud jujucloud.Cloud, force bool) error {
			s.Require().Len(cloud.Regions, 1)
			s.Require().Equal(jujucloud.DefaultCloudRegion, cloud.Regions[0].Name)
			return nil
		}).
		Times(1)

	err := cc.AddCloud(AddCloudInput{})

	s.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestKubernetesCloudSuite(t *testing.T) {
	suite.Run(t, new(CloudSuite))
}
