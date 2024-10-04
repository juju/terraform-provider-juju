package juju

import (
	"github.com/juju/juju/api"
	k8s "github.com/juju/juju/caas/kubernetes"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	"k8s.io/client-go/tools/clientcmd"
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
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

func (s *KubernetesCloudSuite) getKubernetesCloudClient() kubernetesCloudsClient {
	return kubernetesCloudsClient{
		SharedClient: s.JujuSuite.mockSharedClient,
		getKubernetesCloudAPIClient: func(connection api.Connection) KubernetesCloudAPIClient {
			return s.mockKubernetesCloudClient
		},
	}
}

func getFakeCloudConfig() string {
	return `
apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: fake-cert==
    server: https://10.172.195.202:16443
  name: microk8s-cluster
contexts:
- context:
    cluster: microk8s-cluster
    user: admin
  name: microk8s
current-context: microk8s
kind: Config
preferences: {}
users:
- name: admin
  user:
    client-certificate-data: fake-cert==
    client-key-data: fake-key=
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
