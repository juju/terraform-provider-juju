// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"github.com/juju/charm/v11"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apiclient "github.com/juju/juju/api/client/client"
	apiresources "github.com/juju/juju/api/client/resources"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
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
}
