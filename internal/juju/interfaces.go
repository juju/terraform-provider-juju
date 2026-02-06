// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"context"
	"io"

	jaasparams "github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	apiresources "github.com/juju/juju/api/client/resources"
	apisecrets "github.com/juju/juju/api/client/secrets"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	jujucloud "github.com/juju/juju/cloud"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/core/secrets"
	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/domain/deployment/charm"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
)

type SharedClient interface {
	GetControllerVersion(context.Context) (semversion.Number, error)
	AddModel(modelName, modelOwner, modelUUID string, modelType model.ModelType)
	GetConnection(modelUUID *string) (api.Connection, error)
	GetOfferingControllerConn(name string) (api.Connection, error)
	AddOfferingController(name string, conf ControllerConfiguration) error
	IsOfferingController(name string) bool
	ModelType(modelUUID string) (model.ModelType, error)
	ModelOwnerAndName(modelUUID string) (string, string, error)
	ModelStatus(modelUUID string, conn api.Connection) (*params.FullStatus, error)
	RemoveModel(modelUUID string)
	// ModelUUID returns a model's UUID based on the model name and owner.
	// Specifying the owner is optional but recommended. See the docstring
	// of the implementation for more details.
	ModelUUID(modelName, modelOwner string) (string, error)

	Debugf(msg string, additionalFields ...map[string]interface{})
	Errorf(err error, msg string)
	Tracef(msg string, additionalFields ...map[string]interface{})
	Warnf(msg string, additionalFields ...map[string]interface{})

	JujuLogger() *jujuLoggerShim
	WaitForResource() bool
}

type ClientAPIClient interface {
	Status(ctx context.Context, args *apiclient.StatusArgs) (*params.FullStatus, error)
}

type ApplicationAPIClient interface {
	AddUnits(ctx context.Context, args apiapplication.AddUnitsParams) ([]string, error)
	ApplicationsInfo(ctx context.Context, applications []names.ApplicationTag) ([]params.ApplicationInfoResult, error)
	Deploy(ctx context.Context, args apiapplication.DeployArgs) error
	DestroyUnits(ctx context.Context, in apiapplication.DestroyUnitsParams) ([]params.DestroyUnitResult, error)
	DeployFromRepository(ctx context.Context, arg apiapplication.DeployFromRepositoryArg) (apiapplication.DeployInfo, []apiapplication.PendingResourceUpload, []error)
	DestroyApplications(ctx context.Context, in apiapplication.DestroyApplicationsParams) ([]params.DestroyApplicationResult, error)
	Expose(ctx context.Context, application string, exposedEndpoints map[string]params.ExposedEndpoint) error
	Get(ctx context.Context, application string) (*params.ApplicationGetResults, error)
	GetCharmURLOrigin(ctx context.Context, applicationName string) (*charm.URL, apicommoncharm.Origin, error)
	GetConstraints(ctx context.Context, applications ...string) ([]constraints.Value, error)
	MergeBindings(ctx context.Context, req params.ApplicationMergeBindingsArgs) error
	ScaleApplication(ctx context.Context, in apiapplication.ScaleApplicationParams) (params.ScaleApplicationResult, error)
	SetCharm(ctx context.Context, cfg apiapplication.SetCharmConfig) error
	SetConfig(ctx context.Context, application, configYAML string, config map[string]string) error
	UnsetApplicationConfig(ctx context.Context, application string, keys []string) error
	SetConstraints(ctx context.Context, application string, constraints constraints.Value) error
	Unexpose(ctx context.Context, application string, endpoints []string) error
}

type ModelConfigAPIClient interface {
	ModelGet(ctx context.Context) (map[string]interface{}, error)
}

type ResourceAPIClient interface {
	AddPendingResources(ctx context.Context, args apiresources.AddPendingResourcesArgs) ([]string, error)
	ListResources(ctx context.Context, applications []string) ([]resource.ApplicationResources, error)
	Upload(ctx context.Context, application, name, filename, pendingID string, reader io.ReadSeeker) error
	UploadPendingResource(ctx context.Context, args apiresources.UploadPendingResourceArgs) (string, error)
}

type SecretAPIClient interface {
	CreateSecret(ctx context.Context, name, description string, data map[string]string) (string, error)
	ListSecrets(ctx context.Context, reveal bool, filter secrets.Filter) ([]apisecrets.SecretDetails, error)
	UpdateSecret(
		ctx context.Context, uri *secrets.URI, name string, autoPrune *bool,
		newName string, description string, data map[string]string,
	) error
	RemoveSecret(ctx context.Context, uri *secrets.URI, name string, revision *int) error
	GrantSecret(ctx context.Context, uri *secrets.URI, name string, apps []string) ([]error, error)
	RevokeSecret(ctx context.Context, uri *secrets.URI, name string, apps []string) ([]error, error)
}

// JaasAPIClient defines the set of methods that the JAAS API provides.
type JaasAPIClient interface {
	AddModelToController(req *jaasparams.AddModelToControllerRequest) (params.ModelInfo, error)
	ListControllers() ([]jaasparams.ControllerInfo, error)
	ListRelationshipTuples(req *jaasparams.ListRelationshipTuplesRequest) (*jaasparams.ListRelationshipTuplesResponse, error)
	AddRelation(req *jaasparams.AddRelationRequest) error
	RemoveRelation(req *jaasparams.RemoveRelationRequest) error
	AddGroup(req *jaasparams.AddGroupRequest) (jaasparams.AddGroupResponse, error)
	GetGroup(req *jaasparams.GetGroupRequest) (jaasparams.GetGroupResponse, error)
	RenameGroup(req *jaasparams.RenameGroupRequest) error
	RemoveGroup(req *jaasparams.RemoveGroupRequest) error
	AddRole(req *jaasparams.AddRoleRequest) (jaasparams.AddRoleResponse, error)
	GetRole(req *jaasparams.GetRoleRequest) (jaasparams.GetRoleResponse, error)
	RenameRole(req *jaasparams.RenameRoleRequest) error
	RemoveRole(req *jaasparams.RemoveRoleRequest) error
}

// CloudAPIClient defines the methods the Juju API client provides for clouds.
type CloudAPIClient interface {
	AddCloud(ctx context.Context, cloud jujucloud.Cloud, force bool) error
	Cloud(ctx context.Context, tag names.CloudTag) (jujucloud.Cloud, error)
	UpdateCloud(ctx context.Context, cloud jujucloud.Cloud) error
	RemoveCloud(ctx context.Context, cloud string) error
	AddCredential(ctx context.Context, cloud string, credential jujucloud.Credential) error
	UserCredentials(ctx context.Context, user names.UserTag, cloud names.CloudTag) ([]names.CloudCredentialTag, error)
}

// AnnotationsAPIClient defines the set of methods that the Annotations API provides.
type AnnotationsAPIClient interface {
	Get(ctx context.Context, tags []string) ([]params.AnnotationsGetResult, error)
	Set(ctx context.Context, annotations map[string]map[string]string) ([]params.ErrorResult, error)
}
