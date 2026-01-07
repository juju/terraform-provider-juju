// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	jujuclock "github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/cloud"
	k8s "github.com/juju/juju/caas/kubernetes"
	"github.com/juju/juju/caas/kubernetes/clientconfig"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/names/v5"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const (
	// workloadStorageKey is the model config attribute used to specify
	// the storage class for provisioning workload storage.
	workloadStorageKey = "workload-storage"

	// operatorStorageKey is the model config attribute used to specify
	// the storage class for provisioning operator storage.
	operatorStorageKey = "operator-storage"
)

type cloudsClient struct {
	SharedClient

	getCloudAPIClient func(connection api.Connection) CloudAPIClient
}

// AddCloudInput is the input parameters for adding a cloud.
type AddCloudInput struct {
	// Name of the cloud.
	Name string

	// Type is the type of cloud, eg ec2, openstack etc.
	// This is one of the provider names registered with
	// environs.RegisterProvider.
	Type string

	// Description describes the type of cloud.
	Description string

	// AuthTypes are the authentication modes supported by the cloud.
	AuthTypes jujucloud.AuthTypes

	// Endpoint is the default endpoint for the cloud regions, may be
	// overridden by a region.
	Endpoint string

	// IdentityEndpoint is the default identity endpoint for the cloud
	// regions, may be overridden by a region.
	IdentityEndpoint string

	// StorageEndpoint is the default storage endpoint for the cloud
	// regions, may be overridden by a region.
	StorageEndpoint string

	// Regions are the regions available in the cloud.
	//
	// Regions is a slice, and not a map, because order is important.
	// The first region in the slice is the default region for the
	// cloud.
	Regions []jujucloud.Region

	// CACertificates contains an optional list of Certificate
	// Authority certificates to be used to validate certificates
	// of cloud infrastructure components
	// The contents are Base64 encoded x.509 certs.
	CACertificates []string

	// Force indicates whether to force adding the cloud.
	// Some cloud types might not function correctly on certain controllers.
	Force bool
}

// UpdateCloudInput is the input parameters for updating a cloud.
type UpdateCloudInput struct {
	// Name of the cloud.
	Name string

	// Type is the type of cloud, eg ec2, openstack etc.
	// This is one of the provider names registered with
	// environs.RegisterProvider.
	Type string

	// Description describes the type of cloud.
	Description string

	// AuthTypes are the authentication modes supported by the cloud.
	AuthTypes jujucloud.AuthTypes

	// Endpoint is the default endpoint for the cloud regions, may be
	// overridden by a region.
	Endpoint string

	// IdentityEndpoint is the default identity endpoint for the cloud
	// regions, may be overridden by a region.
	IdentityEndpoint string

	// StorageEndpoint is the default storage endpoint for the cloud
	// regions, may be overridden by a region.
	StorageEndpoint string

	// Regions are the regions available in the cloud.
	//
	// Regions is a slice, and not a map, because order is important.
	// The first region in the slice is the default region for the
	// cloud.
	Regions []jujucloud.Region

	// CACertificates contains an optional list of Certificate
	// Authority certificates to be used to validate certificates
	// of cloud infrastructure components
	// The contents are Base64 encoded x.509 certs.
	CACertificates []string
}

// ReadCloudInput is the input parameters for reading a cloud.
type ReadCloudInput struct {
	Name string
}

// ReadCloudOutput is the output parameters for reading a cloud.
type ReadCloudOutput struct {
	// Name of the cloud.
	Name string

	// Type is the type of cloud, eg ec2, openstack etc.
	// This is one of the provider names registered with
	// environs.RegisterProvider.
	Type string

	// Description describes the type of cloud.
	Description string

	// AuthTypes are the authentication modes supported by the cloud.
	AuthTypes jujucloud.AuthTypes

	// Endpoint is the default endpoint for the cloud regions, may be
	// overridden by a region.
	Endpoint string

	// IdentityEndpoint is the default identity endpoint for the cloud
	// regions, may be overridden by a region.
	IdentityEndpoint string

	// StorageEndpoint is the default storage endpoint for the cloud
	// regions, may be overridden by a region.
	StorageEndpoint string

	// Regions are the regions available in the cloud.
	//
	// Regions is a slice, and not a map, because order is important.
	// The first region in the slice is the default region for the
	// cloud.
	Regions []jujucloud.Region

	// CACertificates contains an optional list of Certificate
	// Authority certificates to be used to validate certificates
	// of cloud infrastructure components
	// The contents are PEM encoded CA certificates.
	CACertificates []string
}

// RemoveCloudInput is the input parameters for removing a cloud.
type RemoveCloudInput struct {
	Name string
}

// CreateKubernetesCloudInput creates a new Kubernetes cloud with juju cloud facade.
type CreateKubernetesCloudInput struct {
	Name                 string
	KubernetesConfig     string
	ParentCloudName      string
	ParentCloudRegion    string
	CreateServiceAccount bool
	StorageClassName     string
}

// ReadKubernetesCloudInput reads a Kubernetes cloud with juju cloud facade.
type ReadKubernetesCloudInput struct {
	Name string
}

// ReadKubernetesCloudOutput is the output parameters for reading a Kubernetes cloud.
type ReadKubernetesCloudOutput struct {
	Name              string
	CredentialName    string
	ParentCloudName   string
	ParentCloudRegion string
}

// UpdateKubernetesCloudInput updates a Kubernetes cloud with juju cloud facade.
type UpdateKubernetesCloudInput struct {
	Name                 string
	KubernetesConfig     string
	ParentCloudName      string
	ParentCloudRegion    string
	CreateServiceAccount bool
}

func newCloudsClient(sc SharedClient) *cloudsClient {
	return &cloudsClient{
		SharedClient: sc,
		getCloudAPIClient: func(connection api.Connection) CloudAPIClient {
			return cloud.NewClient(connection)
		},
	}
}

// CreateKubernetesCloud creates a new Kubernetes cloud with juju cloud facade.
// The credential name for this cloud is returned.
func (c *cloudsClient) CreateKubernetesCloud(input *CreateKubernetesCloudInput) (string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

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

	// For the details of storage class skippage, see [provider.StorageClassNameMarkdownDescription].
	if input.StorageClassName != "" {
		newCloud.Config = make(map[string]interface{})
		newCloud.Config[operatorStorageKey] = input.StorageClassName
		newCloud.Config[workloadStorageKey] = input.StorageClassName
	}

	cloudClient := c.getCloudAPIClient(conn)
	err = cloudClient.AddCloud(newCloud, false)
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
	err = cloudClient.AddCredential(cloudCredTag.String(), newCredential)
	if err != nil {
		return "", errors.Annotate(err, "adding kubernetes cloud credential")
	}

	return credentialName, nil
}

// ReadKubernetesCloud reads a Kubernetes cloud with juju cloud facade.
func (c *cloudsClient) ReadKubernetesCloud(input ReadKubernetesCloudInput) (*ReadKubernetesCloudOutput, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	cld, err := cloudClient.Cloud(names.NewCloudTag(input.Name))
	if err != nil {
		return nil, errors.Annotate(err, "getting clouds")
	}

	userName := getCurrentJujuUser(conn)

	cloudCredentialTags, err := cloudClient.UserCredentials(names.NewUserTag(userName), names.NewCloudTag(input.Name))
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

// UpdateKubernetesCloud updates a Kubernetes cloud with juju cloud facade.
func (c *cloudsClient) UpdateKubernetesCloud(input UpdateKubernetesCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)
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

	err = cloudClient.UpdateCloud(newCloud)
	if err != nil {
		return errors.Annotate(err, "updating kubernetes cloud")
	}

	return nil
}

// AddCloud adds a cloud definition to the controller.
func (c *cloudsClient) AddCloud(input AddCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	// All clouds must have at least one default region - lp#1819409.
	if len(input.Regions) == 0 {
		input.Regions = []jujucloud.Region{{Name: jujucloud.DefaultCloudRegion}}
	}

	cloud := jujucloud.Cloud{
		Name:              input.Name,
		Type:              input.Type,
		Description:       input.Description,
		AuthTypes:         input.AuthTypes,
		Endpoint:          input.Endpoint,
		IdentityEndpoint:  input.IdentityEndpoint,
		StorageEndpoint:   input.StorageEndpoint,
		Regions:           input.Regions,
		CACertificates:    encodeB64Certs(input.CACertificates),
		SkipTLSVerify:     false,
		IsControllerCloud: false,
	}

	return cloudClient.AddCloud(cloud, input.Force)
}

// UpdateCloud updates a cloud definition on the controller.
func (c *cloudsClient) UpdateCloud(input UpdateCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	cloud := jujucloud.Cloud{
		Name:              input.Name,
		Type:              input.Type,
		Description:       input.Description,
		AuthTypes:         input.AuthTypes,
		Endpoint:          input.Endpoint,
		IdentityEndpoint:  input.IdentityEndpoint,
		StorageEndpoint:   input.StorageEndpoint,
		Regions:           input.Regions,
		CACertificates:    encodeB64Certs(input.CACertificates),
		SkipTLSVerify:     false,
		IsControllerCloud: false,
	}

	return cloudClient.UpdateCloud(cloud)
}

// RemoveCloud removes a cloud.
func (c *cloudsClient) RemoveCloud(input RemoveCloudInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	return cloudClient.RemoveCloud(input.Name)
}

// ReadCloud reads a cloud.
func (c *cloudsClient) ReadCloud(input ReadCloudInput) (*ReadCloudOutput, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	jjCloud, err := cloudClient.Cloud(names.NewCloudTag(input.Name))
	if err != nil {
		return nil, errors.Annotate(err, "getting cloud")
	}

	decodedCACertificates, decodedCACertificatesErr := decodeB64Certs(jjCloud.CACertificates)
	if decodedCACertificatesErr != nil {
		return nil, errors.Annotate(decodedCACertificatesErr, "decoding cloud CA certificates")
	}

	return &ReadCloudOutput{
		Name:             jjCloud.Name,
		Type:             jjCloud.Type,
		Description:      jjCloud.Description,
		AuthTypes:        jjCloud.AuthTypes,
		Endpoint:         jjCloud.Endpoint,
		IdentityEndpoint: jjCloud.IdentityEndpoint,
		StorageEndpoint:  jjCloud.StorageEndpoint,
		Regions:          jjCloud.Regions,
		CACertificates:   decodedCACertificates,
	}, nil
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

func getNewCredentialUID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Trace(err)
	}
	return hex.EncodeToString(b), nil
}

func encodeB64Certs(cacerts []string) []string {
	encoded := make([]string, len(cacerts))
	for i, cert := range cacerts {
		encoded[i] = base64.StdEncoding.EncodeToString([]byte(cert))
	}
	return encoded
}

func decodeB64Certs(cacerts []string) ([]string, error) {
	decoded := make([]string, len(cacerts))
	for i, cert := range cacerts {
		b, err := base64.StdEncoding.DecodeString(cert)
		if err != nil {
			return nil, fmt.Errorf("failed to base64 decode certificate at index %d: %w", i, err)
		}
		decoded[i] = string(b)
	}
	return decoded, nil
}
