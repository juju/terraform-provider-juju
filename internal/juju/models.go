// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"strings"
	"time"

	"github.com/juju/errors"
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
}

type CreateModelInput struct {
	Name        string
	CloudName   string
	CloudRegion string
	Config      map[string]string
	Credential  string
	Constraints constraints.Value
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
	User            string
	Access          string
	ModelIdentifier string
}

type UpdateAccessModelInput struct {
	ModelIdentifier string
	OldAccess       string
	Grant           []string
	Revoke          []string
	Access          string
}

type DestroyAccessModelInput struct {
	ModelIdentifier string
	Revoke          []string
	Access          string
}

func newModelsClient(sc SharedClient) *modelsClient {
	return &modelsClient{
		SharedClient: sc,
	}
}

// GetModelByName retrieves a model by name
func (c *modelsClient) GetModelByName(name string) (*params.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	modelUUID, err := c.ModelUUID(name)
	if err != nil {
		return nil, err
	}
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

	c.AddModel(modelInfo.Name, modelUUID, model.ModelType(modelInfo.Type))

	c.Tracef(fmt.Sprintf("Retrieved model info: %s, %+v", name, modelInfo))
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

	client := modelmanager.NewClient(conn)

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

	modelInfo, err := client.CreateModel(modelName, currentUser, cloudName, cloudRegion, *cloudCredTag, configValues)
	if err != nil {
		// When we create multiple models concurrently, it can happen that Juju returns an error
		// that the transaction was aborted. We return a specific error here,
		// to make sure we can retry.
		if strings.Contains(err.Error(), "transaction aborted") {
			return resp, TransactionError
		}
		return resp, err
	}

	resp.Cloud = modelInfo.Cloud
	resp.CloudRegion = modelInfo.CloudRegion
	resp.CloudCredentialName = names.NewCloudCredentialTag(modelInfo.CloudCredential).Name()
	resp.Type = modelInfo.Type.String()
	resp.UUID = modelInfo.UUID

	// Add a model object on the client internal to the provider
	c.AddModel(modelInfo.Name, modelInfo.UUID, modelInfo.Type)

	// set constraints when required
	if input.Constraints.String() == "" {
		return resp, nil
	}

	// we have to set constraints ...
	// establish a new connection with the created model through the modelconfig api to set constraints
	connModel, err := c.GetConnection(&modelName)
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

func (c *modelsClient) ReadModel(name string) (*ReadModelResponse, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelmanagerConn.Close() }()

	modelconfigConn, err := c.GetConnection(&name)
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
		return nil, errors.Errorf("Not connected to model %q", name)
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
func (c *modelsClient) ReadModelStatus(name string) (*ReadModelStatusResponse, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = modelmanagerConn.Close() }()

	modelmanagerClient := modelmanager.NewClient(modelmanagerConn)
	modelUUID, err := c.ModelUUID(name)
	if err != nil {
		return nil, err
	}
	modelStatus, err := modelmanagerClient.ModelStatus(names.NewModelTag(modelUUID))
	if err != nil {
		return nil, err
	}
	if len(modelStatus) < 1 {
		return nil, errors.WithType(err, ModelNotFoundError)
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

	modelUUID, err := c.ModelUUID(input.ModelIdentifier)
	if err != nil {
		return err
	}

	err = client.GrantModel(input.User, input.Access, modelUUID)
	if err != nil {
		return err
	}

	return nil
}

// Note we do a revoke against `read` to remove the user from the model access
// If a user has had `write`, then removing that access would decrease their
// access to `read` and the user will remain part of the model access.
func (c *modelsClient) UpdateAccessModel(input UpdateAccessModelInput) error {
	model := input.ModelIdentifier
	access := input.OldAccess

	uuid, err := c.ModelUUID(model)
	if err != nil {
		return err
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	for _, user := range input.Revoke {
		err := client.RevokeModel(user, "read", uuid)
		if err != nil {
			return err
		}
	}

	for _, user := range input.Grant {
		err := client.GrantModel(user, access, uuid)
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

	uuid, err := c.ModelUUID(input.ModelIdentifier)
	if err != nil {
		return err
	}

	for _, user := range input.Revoke {
		err := client.RevokeModel(user, "read", uuid)
		if err != nil {
			return err
		}
	}

	return nil
}
