// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"strings"
	"time"

	jaasapi "github.com/canonical/jimm-go-sdk/v3/api"
	jaasparams "github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/juju/errors"
	jujuapi "github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"
)

// TransactionError is returned when a transaction is aborted.
const TransactionError = errors.ConstError("transaction-aborted")

// ModelNotFoundError is returned when a model cannot be found
// when contacting the Juju API.
var ModelNotFoundError = errors.ConstError("model-not-found")

type modelsClient struct {
	SharedClient
	createModel func(conn jujuapi.Connection,
		name, owner, cloud, cloudRegion string,
		cloudCredential names.CloudCredentialTag,
		config map[string]interface{}, targetController string) (CreateModelResponse, error)
}

type CreateModelInput struct {
	Name             string
	CloudName        string
	CloudRegion      string
	Config           map[string]string
	Credential       string
	Constraints      constraints.Value
	TargetController string
}

type CreateModelResponse struct {
	Cloud               string
	CloudRegion         string
	CloudCredentialName string
	Type                string
	UUID                string
}

type ReadModelResponse struct {
	ModelInfo        params.ModelInfo
	ModelConfig      map[string]interface{}
	ModelConstraints constraints.Value
}

// ReadModelStatusResponse contains the status of a model.
type ReadModelStatusResponse struct {
	ModelStatus base.ModelStatus
}

type UpdateModelInput struct {
	Name        string
	UUID        string
	CloudName   string
	Config      map[string]string
	Unset       []string
	Constraints *constraints.Value
	Credential  string
}

type DestroyModelInput struct {
	UUID string
}

type GrantModelInput struct {
	User      string
	Access    string
	ModelUUID string
}

type UpdateAccessModelInput struct {
	ModelUUID string
	OldAccess string
	Grant     []string
	Revoke    []string
	Access    string
}

type DestroyAccessModelInput struct {
	ModelUUID string
	Revoke    []string
	Access    string
}

func newModelsClient(sc SharedClient, isJAAS bool) *modelsClient {
	if isJAAS {
		return &modelsClient{
			SharedClient: sc,
			createModel:  createJAASModel,
		}
	}
	return &modelsClient{
		SharedClient: sc,
		createModel:  createJujuModel,
	}
}

// GetModel retrieves a model by UUID.
func (c *modelsClient) GetModel(modelUUID string) (*params.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	modelTag := names.NewModelTag(modelUUID)

	results, err := client.ModelInfo([]names.ModelTag{
		modelTag,
	})
	if err != nil {
		return nil, err
	}
	if results[0].Error != nil {
		return nil, results[0].Error
	}

	modelInfo := results[0].Result
	modelOwnerTag, err := names.ParseUserTag(modelInfo.OwnerTag)
	if err != nil {
		return nil, errors.Annotatef(err, "parsing owner tag %q for model %q", modelInfo.OwnerTag, modelInfo.Name)
	}

	c.AddModel(modelInfo.Name, modelOwnerTag.Id(), modelUUID, model.ModelType(modelInfo.Type))

	c.Tracef(fmt.Sprintf("Retrieved model info: %s, %+v", modelUUID, modelInfo))
	return modelInfo, nil
}

func (c *modelsClient) CreateModel(input CreateModelInput) (CreateModelResponse, error) {
	resp := CreateModelResponse{}

	modelName := input.Name
	if !names.IsValidModelName(modelName) {
		return resp, fmt.Errorf("%q is not a valid name: model names may only contain lowercase letters, digits and hyphens", modelName)
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return resp, err
	}
	defer func() { _ = conn.Close() }()

	currentUser := getCurrentJujuUser(conn)

	cloudName := input.CloudName
	cloudRegion := input.CloudRegion

	cloudCredTag := &names.CloudCredentialTag{}
	if input.Credential != "" {
		cloudCredTag, err = GetCloudCredentialTag(cloudName, currentUser, input.Credential)
		if err != nil {
			return resp, err
		}
	}

	// Casting to map[string]interface{} because of client.CreateModel
	configValues := make(map[string]interface{})

	for key, configVal := range input.Config {
		configValues[key] = configVal
	}

	targetController := input.TargetController

	resp, err = c.createModel(conn, modelName, currentUser, cloudName, cloudRegion, *cloudCredTag, configValues, targetController)
	if err != nil {
		// When we create multiple models concurrently, it can happen that Juju returns an error
		// that the transaction was aborted. We return a specific error here,
		// to make sure we can retry.
		if strings.Contains(err.Error(), "transaction aborted") {
			return resp, TransactionError
		}
		return resp, err
	}

	// Add a model object on the client internal to the provider
	c.AddModel(modelName, currentUser, resp.UUID, model.ModelType(resp.Type))

	// set constraints when required
	if input.Constraints.String() == "" {
		return resp, nil
	}

	// we have to set constraints ...
	// establish a new connection with the created model through the modelconfig api to set constraints
	connModel, err := c.GetConnection(&resp.UUID)
	if err != nil {
		return resp, err
	}
	defer func() { _ = conn.Close() }()

	modelClient := modelconfig.NewClient(connModel)
	err = modelClient.SetModelConstraints(input.Constraints)
	if err != nil {
		return resp, err
	}

	return resp, nil
}

// createJAASModel creates a Juju model using the JAAS API client.
// This is required to support creating models on a specific controller.
func createJAASModel(conn jujuapi.Connection,
	name, owner, cloud, cloudRegion string,
	cloudCredential names.CloudCredentialTag,
	config map[string]interface{}, targetController string) (CreateModelResponse, error) {
	var resp CreateModelResponse

	if !names.IsValidUser(owner) {
		return resp, fmt.Errorf("%q is not a valid user name", owner)
	}
	var cloudTag string
	if cloud != "" {
		if !names.IsValidCloud(cloud) {
			return resp, fmt.Errorf("%q is not a valid cloud name", cloud)
		}
		cloudTag = names.NewCloudTag(cloud).String()
	}

	client := jaasapi.NewClient(conn)

	modelInfo, err := client.AddModelToController(&jaasparams.AddModelToControllerRequest{
		ModelCreateArgs: params.ModelCreateArgs{
			Name:               name,
			OwnerTag:           names.NewUserTag(owner).String(),
			CloudTag:           cloudTag,
			CloudRegion:        cloudRegion,
			CloudCredentialTag: cloudCredential.String(),
			Config:             config,
		},
		ControllerName: targetController,
	})
	if err != nil {
		return resp, err
	}

	cloudTagResult, err := names.ParseCloudTag(modelInfo.CloudTag)
	if err != nil {
		return resp, err
	}

	cloudCredentialTag, err := names.ParseCloudCredentialTag(modelInfo.CloudCredentialTag)
	if err != nil {
		return resp, err
	}

	resp.Cloud = cloudTagResult.Id()
	resp.CloudRegion = modelInfo.CloudRegion
	resp.CloudCredentialName = cloudCredentialTag.Name()
	resp.Type = modelInfo.Type
	resp.UUID = modelInfo.UUID

	return resp, nil
}

// createJujuModel creates a Juju model using Juju's modelmanager client.
func createJujuModel(conn jujuapi.Connection,
	name, owner, cloud, cloudRegion string,
	cloudCredential names.CloudCredentialTag,
	config map[string]interface{}, targetController string) (CreateModelResponse, error) {
	if targetController != "" {
		return CreateModelResponse{}, fmt.Errorf("targetController parameter is not supported for Juju model creation")
	}

	var resp CreateModelResponse

	client := modelmanager.NewClient(conn)

	modelInfo, err := client.CreateModel(name, owner, cloud, cloudRegion, cloudCredential, config)
	if err != nil {
		return resp, err
	}

	resp.Cloud = modelInfo.Cloud
	resp.CloudRegion = modelInfo.CloudRegion
	resp.CloudCredentialName = names.NewCloudCredentialTag(modelInfo.CloudCredential).Name()
	resp.Type = modelInfo.Type.String()
	resp.UUID = modelInfo.UUID
	return resp, nil
}

// ListModels retrieves the list of model UUIDs.
func (c *modelsClient) ListModels() ([]string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)
	models, err := client.ListModels(c.GetUser())
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(models))
	for _, model := range models {
		if model.Name == "controller" {
			continue
		}
		ids = append(ids, model.UUID)
	}

	return ids, nil
}

func (c *modelsClient) ReadModel(modelUUID string) (*ReadModelResponse, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelmanagerConn.Close() }()

	modelconfigConn, err := c.GetConnection(&modelUUID)
	if err != nil {
		if params.IsCodeNotFound(err) {
			return nil, errors.WithType(err, ModelNotFoundError)
		}
		return nil, err
	}
	defer func() { _ = modelconfigConn.Close() }()

	modelmanagerClient := modelmanager.NewClient(modelmanagerConn)
	modelconfigClient := modelconfig.NewClient(modelconfigConn)

	modelUUIDTag, modelOk := modelconfigConn.ModelTag()
	if !modelOk {
		return nil, errors.Errorf("Not connected to model %q", modelUUID)
	}
	models, err := modelmanagerClient.ModelInfo([]names.ModelTag{modelUUIDTag})
	if err != nil {
		return nil, err
	}

	if len(models) > 1 {
		return nil, fmt.Errorf("more than one model returned for UUID: %s", modelUUIDTag.Id())
	}
	if len(models) < 1 {
		return nil, ModelNotFoundError
	}

	// Check if the model has an error first
	if models[0].Error != nil {
		return nil, models[0].Error
	}
	modelInfo := *models[0].Result

	modelConfig, err := modelconfigClient.ModelGet()
	if err != nil {
		return nil, err
	}

	modelConstraints, err := modelconfigClient.GetModelConstraints()
	if err != nil {
		return nil, err
	}

	return &ReadModelResponse{
		ModelInfo:        modelInfo,
		ModelConfig:      modelConfig,
		ModelConstraints: modelConstraints,
	}, nil
}

// ReadModelStatus retrieves the status of a model by its name.
// It returns a ReadModelStatusResponse containing the model's status.
// If the model is not found, it returns an error.
func (c *modelsClient) ReadModelStatus(modelUUID string) (*ReadModelStatusResponse, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelmanagerConn.Close() }()

	modelmanagerClient := modelmanager.NewClient(modelmanagerConn)
	modelStatus, err := modelmanagerClient.ModelStatus(names.NewModelTag(modelUUID))
	if err != nil {
		return nil, err
	}
	if len(modelStatus) < 1 {
		return nil, errors.WithType(errors.New("no models found"), ModelNotFoundError)
	}
	if len(modelStatus) > 1 {
		return nil, fmt.Errorf("more than one model returned for UUID: %s", modelUUID)
	}
	if modelStatus[0].Error != nil {
		if params.IsCodeNotFound(modelStatus[0].Error) {
			return nil, errors.WithType(modelStatus[0].Error, ModelNotFoundError)
		}
		return nil, modelStatus[0].Error
	}

	return &ReadModelStatusResponse{
		ModelStatus: modelStatus[0],
	}, nil
}

func (c *modelsClient) UpdateModel(input UpdateModelInput) error {
	conn, err := c.GetConnection(&input.UUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelconfig.NewClient(conn)

	configMap := make(map[string]interface{})
	for key, value := range input.Config {
		configMap[key] = value
	}
	if input.Config != nil {
		err = client.ModelSet(configMap)
		if err != nil {
			return err
		}
	}

	if input.Unset != nil {
		err = client.ModelUnset(input.Unset...)
		if err != nil {
			return err
		}
	}

	if input.Constraints != nil {
		err = client.SetModelConstraints(*input.Constraints)
		if err != nil {
			return err
		}
	}

	if input.Credential != "" {
		cloudName := input.CloudName
		currentUser := getCurrentJujuUser(conn)
		cloudCredTag, err := GetCloudCredentialTag(cloudName, currentUser, input.Credential)
		if err != nil {
			return err
		}
		// open new connection to get facade versions correctly
		connModelManager, err := c.GetConnection(nil)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		modelUUIDTag, modelOk := conn.ModelTag()
		if !modelOk {
			return errors.Errorf("Not connected to model %q", input.Name)
		}
		clientModelManager := modelmanager.NewClient(connModelManager)
		if err := clientModelManager.ChangeModelCredential(modelUUIDTag, *cloudCredTag); err != nil {
			return err
		}
	}

	return nil
}

func (c *modelsClient) DestroyModel(input DestroyModelInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	maxWait := 10 * time.Minute
	timeout := 30 * time.Minute

	tag := names.NewModelTag(input.UUID)

	destroyStorage := true
	forceDestroy := false

	err = client.DestroyModel(tag, &destroyStorage, &forceDestroy, &maxWait, &timeout)
	if err != nil {
		return err
	}

	c.RemoveModel(input.UUID)
	return nil
}

func (c *modelsClient) GrantModel(input GrantModelInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	err = client.GrantModel(input.User, input.Access, input.ModelUUID)
	if err != nil {
		return err
	}

	return nil
}

// Note we do a revoke against `read` to remove the user from the model access
// If a user has had `write`, then removing that access would decrease their
// access to `read` and the user will remain part of the model access.
func (c *modelsClient) UpdateAccessModel(input UpdateAccessModelInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	for _, user := range input.Revoke {
		err := client.RevokeModel(user, "read", input.ModelUUID)
		if err != nil {
			return err
		}
	}

	for _, user := range input.Grant {
		err := client.GrantModel(user, input.OldAccess, input.ModelUUID)
		if err != nil {
			return err
		}
	}

	return nil
}

// Note we do a revoke against `read` to remove the user from the model access
// If a user has had `write`, then removing that access would decrease their
// access to `read` and the user will remain part of the model access.
func (c *modelsClient) DestroyAccessModel(input DestroyAccessModelInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	for _, user := range input.Revoke {
		err := client.RevokeModel(user, "read", input.ModelUUID)
		if err != nil {
			return err
		}
	}

	return nil
}
