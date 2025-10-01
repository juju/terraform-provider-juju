// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"crypto/rand"
	"encoding/hex"
	"strings"

	jujuclock "github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/cloud"
	k8s "github.com/juju/juju/caas/kubernetes"
	"github.com/juju/juju/caas/kubernetes/clientconfig"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	"github.com/juju/names/v5"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// WorkloadStorageKey is the model config attribute used to specify
	// the storage class for provisioning workload storage.
	WorkloadStorageKey = "workload-storage"

	// OperatorStorageKey is the model config attribute used to specify
	// the storage class for provisioning operator storage.
	OperatorStorageKey = "operator-storage"
)

type kubernetesCloudsClient struct {
	SharedClient

	getKubernetesCloudAPIClient func(connection api.Connection) KubernetesCloudAPIClient
}

type CreateKubernetesCloudInput struct {
	Name                 string
	KubernetesConfig     string
	ParentCloudName      string
	ParentCloudRegion    string
	CreateServiceAccount bool
	StorageClassName     string
}

type ReadKubernetesCloudInput struct {
	Name string
}

type ReadKubernetesCloudOutput struct {
	Name              string
	CredentialName    string
	ParentCloudName   string
	ParentCloudRegion string
}

type UpdateKubernetesCloudInput struct {
	Name                 string
	KubernetesConfig     string
	ParentCloudName      string
	ParentCloudRegion    string
	CreateServiceAccount bool
}

type DestroyKubernetesCloudInput struct {
	Name string
}

func newKubernetesCloudsClient(sc SharedClient) *kubernetesCloudsClient {
	return &kubernetesCloudsClient{
		SharedClient: sc,
		getKubernetesCloudAPIClient: func(connection api.Connection) KubernetesCloudAPIClient {
			return cloud.NewClient(connection)
		},
	}
}

func getNewCredentialUID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Trace(err)
	}
	return hex.EncodeToString(b), nil
}

// CreateKubernetesCloud creates a new Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) CreateKubernetesCloud(input *CreateKubernetesCloudInput) (string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	kubernetesAPIClient := c.getKubernetesCloudAPIClient(conn)
	k8sConf, err := createKubernetesConfig([]byte(input.KubernetesConfig), input.CreateServiceAccount)
	if err != nil {
		return "", errors.Annotate(err, "parsing kubernetes configuration data")
	}

	var hostCloudRegion string
	if input.ParentCloudName != "" || input.ParentCloudRegion != "" {
		hostCloudRegion = input.ParentCloudName + "/" + input.ParentCloudRegion
	} else {
		hostCloudRegion = k8s.K8sCloudOther
	}
	newCloud, err := k8scloud.CloudFromKubeConfigContext(
		k8sConf.CurrentContext,
		k8sConf,
		k8scloud.CloudParamaters{
			Name:            input.Name,
			HostCloudRegion: hostCloudRegion,
		},
	)
	if err != nil {
		return "", errors.Trace(err)
	}

	// Setup storage.
	//
	// The Juju CLI's add-k8s command performs intelligent storage class selection when adding
	// a Kubernetes cloud. If a storage class is specified via --storage flag, Juju
	// validates that the named storage class exists in the cluster and uses it for BOTH
	// operator storage and workload storage.
	// If no storage class is specified, Juju automatically selects the best available
	// storage classes based on cloud provider preferences (e.g., 'gp2' for AWS, 'standard'
	// for GCE). We are not going to implement this intelligent selection as it requires
	// direct communication with the Kubernetes cluster in question to be added as a cloud.
	// That is, when running terraform and attempting to add a Kubernetes cloud, the caller
	// would need network connectivity to the cluster.
	//
	// Instead, we expect users to explicitly define the storage class names to use for
	// operator and workload storage.
	//
	// Furthermore, there's no need to worry when updating Kubernetes cloud defintions as
	// Juju does not allow for this.
	//
	// Lastly, we check for the existence of the key and are keeping it optional as Juju
	// has a --skip-storage key. In effect, we skip storage by not supplying
	// the [CreateKubernetesCloudInput.StorageClassName].
	if input.StorageClassName != "" {
		newCloud.Config = make(map[string]interface{})
		newCloud.Config[OperatorStorageKey] = input.StorageClassName
		newCloud.Config[WorkloadStorageKey] = input.StorageClassName
	}

	err = kubernetesAPIClient.AddCloud(newCloud, false)
	if err != nil {
		return "", errors.Annotate(err, "adding kubernetes cloud")
	}

	credentialName := input.Name
	cloudName := input.Name

	currentUser := getCurrentJujuUser(conn)

	cloudCredTag, err := GetCloudCredentialTag(cloudName, currentUser, credentialName)
	if err != nil {
		return "", errors.Annotate(err, "getting cloud credential tag")
	}

	newCredential, err := k8scloud.CredentialFromKubeConfigContext(k8sConf.CurrentContext, k8sConf)
	if err != nil {
		return "", errors.Trace(err)
	}
	err = kubernetesAPIClient.AddCredential(cloudCredTag.String(), newCredential)
	if err != nil {
		return "", errors.Annotate(err, "adding kubernetes cloud credential")
	}

	return credentialName, nil
}

// createKubernetesConfig creates a Kubernetes configuration from the provided config data.
// If createServiceAccount is true, it will create or get the Juju admin service account credentials.
// If createServiceAccount is false, it will use the credentials already present in the config data.
func createKubernetesConfig(config []byte, createServiceAccount bool) (*clientcmdapi.Config, error) {
	conf, err := clientcmd.NewClientConfigFromBytes(config)
	if err != nil {
		return nil, errors.Annotate(err, "parsing kubernetes configuration data")
	}

	k8sConf, err := conf.RawConfig()
	if err != nil {
		return nil, errors.Annotate(err, "fetching kubernetes configuration")
	}

	if !createServiceAccount {
		return &k8sConf, nil
	}

	// If createServiceAccount is true, we need to create or get the Juju admin service account credentials and update the config.
	credentialUUID, err := getNewCredentialUID()
	if err != nil {
		return nil, errors.Annotate(err, "generating new credential UID")
	}
	credResolver := clientconfig.GetJujuAdminServiceAccountResolver(jujuclock.WallClock)
	k8sConfWithCreds, err := credResolver(credentialUUID, &k8sConf, k8sConf.CurrentContext)
	if err != nil {
		return nil, errors.Annotate(err, "resolving k8s credential")
	}

	return k8sConfWithCreds, nil
}

// ReadKubernetesCloud reads a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) ReadKubernetesCloud(input ReadKubernetesCloudInput) (*ReadKubernetesCloudOutput, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	kubernetesAPIClient := c.getKubernetesCloudAPIClient(conn)

	cld, err := kubernetesAPIClient.Cloud(names.NewCloudTag(input.Name))
	if err != nil {
		return nil, errors.Annotate(err, "getting clouds")
	}

	userName := getCurrentJujuUser(conn)

	cloudCredentialTags, err := kubernetesAPIClient.UserCredentials(names.NewUserTag(userName), names.NewCloudTag(input.Name))
	if err != nil {
		return nil, errors.Annotate(err, "getting user credentials")
	}
	if len(cloudCredentialTags) == 0 {
		return nil, errors.NotFoundf("cloud credentials for user %q", userName)
	}

	credentialName := cloudCredentialTags[0].Name()

	parentCloudName, parentCloudRegion := getParentCloudNameAndRegion(cld.HostCloudRegion)
	return &ReadKubernetesCloudOutput{
		Name:              input.Name,
		CredentialName:    credentialName,
		ParentCloudName:   parentCloudName,
		ParentCloudRegion: parentCloudRegion,
	}, nil
}

// getParentCloudNameAndRegion returns the parent cloud name
// and region from the host cloud region. HostCloudRegion represents the k8s
// host cloud. The format is <cloudName>/<region>.
func getParentCloudNameAndRegion(hostCloudRegion string) (string, string) {
	parts := strings.Split(hostCloudRegion, "/")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// UpdateKubernetesCloud updates a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) UpdateKubernetesCloud(input UpdateKubernetesCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	kubernetesAPIClient := c.getKubernetesCloudAPIClient(conn)
	k8sConf, err := createKubernetesConfig([]byte(input.KubernetesConfig), input.CreateServiceAccount)
	if err != nil {
		return errors.Annotate(err, "parsing kubernetes configuration data")
	}

	var hostCloudRegion string
	if input.ParentCloudName != "" || input.ParentCloudRegion != "" {
		hostCloudRegion = input.ParentCloudName + "/" + input.ParentCloudRegion
	} else {
		hostCloudRegion = k8s.K8sCloudOther
	}

	newCloud, err := k8scloud.CloudFromKubeConfigContext(
		k8sConf.CurrentContext,
		k8sConf,
		k8scloud.CloudParamaters{
			Name:            input.Name,
			HostCloudRegion: hostCloudRegion,
		},
	)
	if err != nil {
		return errors.Trace(err)
	}

	err = kubernetesAPIClient.UpdateCloud(newCloud)
	if err != nil {
		return errors.Annotate(err, "updating kubernetes cloud")
	}

	return nil
}

// RemoveKubernetesCloud removes a Kubernetes cloud with juju cloud facade.
func (c *kubernetesCloudsClient) RemoveKubernetesCloud(input DestroyKubernetesCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	kubernetesAPIClient := c.getKubernetesCloudAPIClient(conn)

	err = kubernetesAPIClient.RemoveCloud(input.Name)
	if err != nil {
		return errors.Annotate(err, "removing kubernetes cloud")
	}

	return nil
}
