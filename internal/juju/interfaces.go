// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	jujucloud "github.com/juju/juju/cloud"
	"io"

	jaasparams "github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/juju/charm/v12"
	charmresources "github.com/juju/charm/v12/resource"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	apiresources "github.com/juju/juju/api/client/resources"
	apisecrets "github.com/juju/juju/api/client/secrets"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"
)

type SharedClient interface {
	AddModel(modelName, modelUUID string, modelType model.ModelType)
	GetConnection(modelName *string) (api.Connection, error)
	ModelType(modelName string) (model.ModelType, error)
	ModelUUID(modelName string) (string, error)
	RemoveModel(modelUUID string)

	Debugf(msg string, additionalFields ...map[string]interface{})
	Errorf(err error, msg string)
	Tracef(msg string, additionalFields ...map[string]interface{})
	Warnf(msg string, additionalFields ...map[string]interface{})

	JujuLogger() *jujuLoggerShim
}

type ClientAPIClient interface {
	Status(args *apiclient.StatusArgs) (*params.FullStatus, error)
}

type ApplicationAPIClient interface {
	AddUnits(args apiapplication.AddUnitsParams) ([]string, error)
	ApplicationsInfo(applications []names.ApplicationTag) ([]params.ApplicationInfoResult, error)
	Deploy(args apiapplication.DeployArgs) error
	DestroyUnits(in apiapplication.DestroyUnitsParams) ([]params.DestroyUnitResult, error)
	DeployFromRepository(arg apiapplication.DeployFromRepositoryArg) (apiapplication.DeployInfo, []apiapplication.PendingResourceUpload, []error)
	DestroyApplications(in apiapplication.DestroyApplicationsParams) ([]params.DestroyApplicationResult, error)
	Expose(application string, exposedEndpoints map[string]params.ExposedEndpoint) error
	Get(branchName, application string) (*params.ApplicationGetResults, error)
	GetCharmURLOrigin(branchName, applicationName string) (*charm.URL, apicommoncharm.Origin, error)
	GetConstraints(applications ...string) ([]constraints.Value, error)
	MergeBindings(req params.ApplicationMergeBindingsArgs) error
	ScaleApplication(in apiapplication.ScaleApplicationParams) (params.ScaleApplicationResult, error)
	SetCharm(branchName string, cfg apiapplication.SetCharmConfig) error
	SetConfig(branchName, application, configYAML string, config map[string]string) error
	SetConstraints(application string, constraints constraints.Value) error
	Unexpose(application string, endpoints []string) error
}

type ModelConfigAPIClient interface {
	ModelGet() (map[string]interface{}, error)
}

type ResourceAPIClient interface {
	AddPendingResources(args apiresources.AddPendingResourcesArgs) ([]string, error)
	ListResources(applications []string) ([]resources.ApplicationResources, error)
	Upload(application, name, filename, pendingID string, reader io.ReadSeeker) error
	UploadPendingResource(applicationID string, resource charmresources.Resource, filename string, r io.ReadSeeker) (id string, err error)
}

type SecretAPIClient interface {
	CreateSecret(name, description string, data map[string]string) (string, error)
	ListSecrets(reveal bool, filter secrets.Filter) ([]apisecrets.SecretDetails, error)
	UpdateSecret(
		uri *secrets.URI, name string, autoPrune *bool,
		newName string, description string, data map[string]string,
	) error
	RemoveSecret(uri *secrets.URI, name string, revision *int) error
	GrantSecret(uri *secrets.URI, name string, apps []string) ([]error, error)
	RevokeSecret(uri *secrets.URI, name string, apps []string) ([]error, error)
}

// JaasAPIClient defines the set of methods that the JAAS API provides.
type JaasAPIClient interface {
	ListRelationshipTuples(req *jaasparams.ListRelationshipTuplesRequest) (*jaasparams.ListRelationshipTuplesResponse, error)
	AddRelation(req *jaasparams.AddRelationRequest) error
	RemoveRelation(req *jaasparams.RemoveRelationRequest) error
	AddGroup(req *jaasparams.AddGroupRequest) (jaasparams.AddGroupResponse, error)
	GetGroup(req *jaasparams.GetGroupRequest) (jaasparams.GetGroupResponse, error)
	RenameGroup(req *jaasparams.RenameGroupRequest) error
	RemoveGroup(req *jaasparams.RemoveGroupRequest) error
}

// KubernetesCloudAPIClient defines the set of methods that the Kubernetes cloud API provides.
type KubernetesCloudAPIClient interface {
	AddCloud(cloud jujucloud.Cloud, force bool) error
	Cloud(tag names.CloudTag) (jujucloud.Cloud, error)
	UpdateCloud(cloud jujucloud.Cloud) error
	RemoveCloud(cloud string) error
}
