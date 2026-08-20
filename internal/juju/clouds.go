// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"

	jaasapi "github.com/canonical/jimm-go-sdk/v3/api"
	jaasparams "github.com/canonical/jimm-go-sdk/v3/api/params"
	jujuclock "github.com/juju/clock"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/cloud"
	k8s "github.com/juju/juju/caas/kubernetes"
	"github.com/juju/juju/caas/kubernetes/clientconfig"
	k8scloud "github.com/juju/juju/caas/kubernetes/cloud"
	jujucloud "github.com/juju/juju/cloud"
	jujuparams "github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
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

var (
	// CloudNotFoundError is returned when a cloud does not exist.
	CloudNotFoundError = errors.New("cloud not found")
)

type cloudsClient struct {
	SharedClient

	isJAAS bool

	getCloudAPIClient func(connection api.Connection) CloudAPIClient
	getJaasApiClient  func(connection api.Connection) JaasAPIClient
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

	// TargetController is the name of the backing controller to which the
	// cloud should be added. It is only used when running against JAAS. If
	// left empty, JIMM selects a controller.
	TargetController string
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

	// TargetController is the name of the backing controller from which the
	// cloud should be removed. It is only used when running against JAAS.
	TargetController string
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

func newCloudsClient(sc SharedClient, isJAAS bool) *cloudsClient {
	return &cloudsClient{
		SharedClient: sc,
		isJAAS:       isJAAS,
		getCloudAPIClient: func(connection api.Connection) CloudAPIClient {
			return cloud.NewClient(connection)
		},
		getJaasApiClient: func(connection api.Connection) JaasAPIClient {
			return jaasapi.NewClient(JaasConnShim{Connection: connection})
		},
	}
}

// cloudToParams converts a jujucloud.Cloud into the wire params.Cloud
// representation expected by the JIMM AddCloudToController facade. This mirrors
// the (unexported) helper in github.com/juju/juju/api/client/cloud.
func cloudToParams(cloud jujucloud.Cloud) jujuparams.Cloud {
	authTypes := make([]string, len(cloud.AuthTypes))
	for i, authType := range cloud.AuthTypes {
		authTypes[i] = string(authType)
	}
	regions := make([]jujuparams.CloudRegion, len(cloud.Regions))
	for i, region := range cloud.Regions {
		regions[i] = jujuparams.CloudRegion{
			Name:             region.Name,
			Endpoint:         region.Endpoint,
			IdentityEndpoint: region.IdentityEndpoint,
			StorageEndpoint:  region.StorageEndpoint,
		}
	}
	var regionConfig map[string]map[string]interface{}
	for r, attr := range cloud.RegionConfig {
		if regionConfig == nil {
			regionConfig = make(map[string]map[string]interface{})
		}
		regionConfig[r] = attr
	}
	return jujuparams.Cloud{
		Type:              cloud.Type,
		HostCloudRegion:   cloud.HostCloudRegion,
		AuthTypes:         authTypes,
		Endpoint:          cloud.Endpoint,
		IdentityEndpoint:  cloud.IdentityEndpoint,
		StorageEndpoint:   cloud.StorageEndpoint,
		Regions:           regions,
		CACertificates:    cloud.CACertificates,
		SkipTLSVerify:     cloud.SkipTLSVerify,
		Config:            cloud.Config,
		RegionConfig:      regionConfig,
		IsControllerCloud: cloud.IsControllerCloud,
	}
}

// CreateKubernetesCloud creates a new Kubernetes cloud with juju cloud facade.
// The credential name for this cloud is returned.
func (c *cloudsClient) CreateKubernetesCloud(ctx context.Context, input *CreateKubernetesCloudInput) (string, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	k8sConf, err := createKubernetesConfig(ctx, []byte(input.KubernetesConfig), input.CreateServiceAccount)
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
	err = cloudClient.AddCloud(ctx, newCloud, false)
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
	err = cloudClient.AddCredential(ctx, cloudCredTag.String(), newCredential)
	if err != nil {
		return "", errors.Annotate(err, "adding kubernetes cloud credential")
	}

	return credentialName, nil
}

// ReadKubernetesCloud reads a Kubernetes cloud with juju cloud facade.
func (c *cloudsClient) ReadKubernetesCloud(ctx context.Context, input ReadKubernetesCloudInput) (*ReadKubernetesCloudOutput, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	cld, err := cloudClient.Cloud(ctx, names.NewCloudTag(input.Name))
	if err != nil {
		return nil, errors.Annotate(err, "getting clouds")
	}

	userName := getCurrentJujuUser(conn)

	cloudCredentialTags, err := cloudClient.UserCredentials(ctx, names.NewUserTag(userName), names.NewCloudTag(input.Name))
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
func (c *cloudsClient) UpdateKubernetesCloud(ctx context.Context, input UpdateKubernetesCloudInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)
	k8sConf, err := createKubernetesConfig(ctx, []byte(input.KubernetesConfig), input.CreateServiceAccount)
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

	err = cloudClient.UpdateCloud(ctx, newCloud)
	if err != nil {
		return errors.Annotate(err, "updating kubernetes cloud")
	}

	return nil
}

// AddCloud adds a cloud definition to the controller.
func (c *cloudsClient) AddCloud(ctx context.Context, input AddCloudInput) error {
	conn, err := c.GetConnection(ctx, nil)
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

	if c.isJAAS {
		// When running against JAAS, the plain Juju AddCloud facade is not
		// appropriate. Instead, we use the JIMM AddCloudToController facade
		// which adds the cloud to a specific backing controller.
		jaasClient := c.getJaasApiClient(conn)
		return jaasClient.AddCloudToController(&jaasparams.AddCloudToControllerRequest{
			AddCloudArgs: jujuparams.AddCloudArgs{
				Name:  input.Name,
				Cloud: cloudToParams(cloud),
				Force: &input.Force,
			},
			ControllerName: input.TargetController,
		})
	}

	return cloudClient.AddCloud(ctx, cloud, input.Force)
}

// UpdateCloud updates a cloud definition on the controller.
func (c *cloudsClient) UpdateCloud(ctx context.Context, input UpdateCloudInput) error {
	conn, err := c.GetConnection(ctx, nil)
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

	return cloudClient.UpdateCloud(ctx, cloud)
}

// RemoveCloud removes a cloud.
func (c *cloudsClient) RemoveCloud(ctx context.Context, input RemoveCloudInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	if c.isJAAS {
		// When running against JAAS, use the JIMM RemoveCloudFromController
		// facade to remove the cloud from its backing controller.
		jaasClient := c.getJaasApiClient(conn)
		return jaasClient.RemoveCloudFromController(&jaasparams.RemoveCloudFromControllerRequest{
			CloudTag:       names.NewCloudTag(input.Name).String(),
			ControllerName: input.TargetController,
		})
	}

	cloudClient := c.getCloudAPIClient(conn)

	return cloudClient.RemoveCloud(ctx, input.Name)
}

// RemoveKubernetesCloud removes a Kubernetes cloud using the plain Juju cloud
// facade. Unlike RemoveCloud, this does not use the JIMM facade when running
// against JAAS, since a Kubernetes cloud is hosted and managed through its
// parent cloud.
func (c *cloudsClient) RemoveKubernetesCloud(ctx context.Context, input RemoveCloudInput) error {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	return cloudClient.RemoveCloud(ctx, input.Name)
}

// ReadCloud reads a cloud.
func (c *cloudsClient) ReadCloud(ctx context.Context, input ReadCloudInput) (*ReadCloudOutput, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	jjCloud, err := cloudClient.Cloud(ctx, names.NewCloudTag(input.Name))
	if errors.Is(err, errors.NotFound) {
		return nil, CloudNotFoundError
	}

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

// ListClouds returns the names of all clouds available on the controller.
func (c *cloudsClient) ListClouds(ctx context.Context) ([]string, error) {
	conn, err := c.GetConnection(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	cloudClient := c.getCloudAPIClient(conn)

	jjClouds, err := cloudClient.Clouds(ctx)
	if err != nil {
		return nil, errors.Annotate(err, "getting clouds")
	}

	clouds := make([]string, 0, len(jjClouds))
	for cloudTag := range jjClouds {
		clouds = append(clouds, cloudTag.Id())
	}

	return clouds, nil
}

// createKubernetesConfig creates a Kubernetes configuration from the provided config data.
// If createServiceAccount is true, it will create or get the Juju admin service account credentials.
// If createServiceAccount is false, it will use the credentials already present in the config data.
func createKubernetesConfig(ctx context.Context, config []byte, createServiceAccount bool) (*clientcmdapi.Config, error) {
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
	credResolver := clientconfig.GetJujuAdminServiceAccountResolver(ctx, jujuclock.WallClock)
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
