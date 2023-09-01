// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

var ModelNotFoundError = &modelNotFoundError{}

type modelNotFoundError struct {
	uuid string
	name string
}

func (me *modelNotFoundError) Error() string {
	toReturn := "model %s was not found"
	if me.name != "" {
		return fmt.Sprintf(toReturn, me.name)
	}
	return fmt.Sprintf(toReturn, me.uuid)
}

type modelsClient struct {
	SharedClient
}

type GrantModelInput struct {
	User       string
	Access     string
	ModelUUIDs []string
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
	ModelInfo base.ModelInfo
}

type ReadModelInput struct {
	UUID string
}

type ReadModelResponse struct {
	ModelInfo        params.ModelInfo
	ModelConfig      map[string]interface{}
	ModelConstraints constraints.Value
}

type UpdateModelInput struct {
	UUID        string
	CloudName   string
	Config      map[string]string
	Unset       []string
	Constraints *constraints.Value
	Credential  string
}

type UpdateAccessModelInput struct {
	ModelName string
	OldAccess string
	Grant     []string
	Revoke    []string
	Access    string
}

type DestroyModelInput struct {
	UUID string
}

type DestroyAccessModelInput struct {
	Model  string
	Revoke []string
	Access string
}

func newModelsClient(sc SharedClient) *modelsClient {
	return &modelsClient{
		SharedClient: sc,
	}
}

func (c *modelsClient) resolveModelUUIDWithClient(client modelmanager.Client, name string, user string) (string, error) {
	modelUUID := ""
	modelSummaries, err := client.ListModelSummaries(user, false)
	if err != nil {
		return "", err
	}
	for _, modelSummary := range modelSummaries {
		if modelSummary.Name == name {
			modelUUID = modelSummary.UUID
			break
		}
	}

	if modelUUID == "" {
		return "", fmt.Errorf("model not found for defined user")
	}

	return modelUUID, nil
}

// GetModelByName retrieves a model by name
func (c *modelsClient) GetModelByName(name string) (*params.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	currentUser := getCurrentJujuUser(conn)
	client := modelmanager.NewClient(conn)
	defer client.Close()

	modelUUID, err := c.resolveModelUUIDWithClient(*client, name, currentUser)
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

	c.Tracef(fmt.Sprintf("Retrieved model info: %s, %+v", name, modelInfo))
	return modelInfo, nil
}

// ResolveModelUUID retrieves a model's using its name
func (c *modelsClient) ResolveModelUUID(name string) (string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return "", err
	}

	currentUser := getCurrentJujuUser(conn)
	client := modelmanager.NewClient(conn)
	defer client.Close()

	modelUUID, err := c.resolveModelUUIDWithClient(*client, name, currentUser)
	if err != nil {
		return "", nil
	}

	return modelUUID, nil
}

func (c *modelsClient) CreateModel(input CreateModelInput) (*CreateModelResponse, error) {
	modelName := input.Name
	if !names.IsValidModelName(modelName) {
		return nil, fmt.Errorf("%q is not a valid name: model names may only contain lowercase letters, digits and hyphens", modelName)
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	currentUser := getCurrentJujuUser(conn)

	client := modelmanager.NewClient(conn)
	defer client.Close()

	cloudName := input.CloudName
	cloudRegion := input.CloudRegion

	cloudCredTag := &names.CloudCredentialTag{}
	if input.Credential != "" {
		cloudCredTag, err = GetCloudCredentialTag(cloudName, currentUser, input.Credential)
		if err != nil {
			return nil, err
		}
	}

	// Casting to map[string]interface{} because of client.CreateModel
	configValues := make(map[string]interface{})

	for key, configVal := range input.Config {
		configValues[key] = configVal
	}

	modelInfo, err := client.CreateModel(modelName, currentUser, cloudName, cloudRegion, *cloudCredTag, configValues)
	if err != nil {
		return nil, err
	}

	// set constraints when required
	if input.Constraints.String() == "" {
		return &CreateModelResponse{ModelInfo: modelInfo}, nil
	}

	// we have to set constraints ...
	// establish a new connection with the created model through the modelconfig api to set constraints
	connModel, err := c.GetConnection(&modelInfo.UUID)
	if err != nil {
		return nil, err
	}

	modelClient := modelconfig.NewClient(connModel)
	defer func() { _ = modelClient.Close() }()
	err = modelClient.SetModelConstraints(input.Constraints)
	if err != nil {
		return nil, err
	}

	return &CreateModelResponse{ModelInfo: modelInfo}, nil
}

func (c *modelsClient) ReadModel(uuid string) (*ReadModelResponse, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	modelconfigConn, err := c.GetConnection(&uuid)
	if err != nil {
		return nil, errors.Join(err, &modelNotFoundError{uuid: uuid})
	}

	modelmanagerClient := modelmanager.NewClient(modelmanagerConn)
	defer modelmanagerClient.Close()

	modelconfigClient := modelconfig.NewClient(modelconfigConn)
	defer modelconfigClient.Close()

	models, err := modelmanagerClient.ModelInfo([]names.ModelTag{names.NewModelTag(uuid)})
	if err != nil {
		return nil, err
	}

	if len(models) > 1 {
		return nil, fmt.Errorf("more than one model returned for UUID: %s", uuid)
	}
	if len(models) < 1 {
		return nil, &modelNotFoundError{uuid: uuid}
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

func (c *modelsClient) UpdateModel(input UpdateModelInput) error {
	conn, err := c.GetConnection(&input.UUID)
	if err != nil {
		return err
	}

	client := modelconfig.NewClient(conn)
	defer client.Close()

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
		tag := names.NewModelTag(input.UUID)
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
		clientModelManager := modelmanager.NewClient(connModelManager)
		defer clientModelManager.Close()
		if err := clientModelManager.ChangeModelCredential(tag, *cloudCredTag); err != nil {
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

	client := modelmanager.NewClient(conn)
	defer client.Close()

	maxWait := 10 * time.Minute
	timeout := 30 * time.Minute

	tag := names.NewModelTag(input.UUID)

	destroyStorage := true
	forceDestroy := false

	err = client.DestroyModel(tag, &destroyStorage, &forceDestroy, &maxWait, timeout)
	if err != nil {
		return err
	}

	return nil
}

func (c *modelsClient) GrantModel(input GrantModelInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	err = client.GrantModel(input.User, input.Access, input.ModelUUIDs...)
	if err != nil {
		return err
	}

	return nil
}

// Note we do a revoke against `read` to remove the user from the model access
// If a user has had `write`, then removing that access would decrease their
// access to `read` and the user will remain part of the model access.
func (c *modelsClient) UpdateAccessModel(input UpdateAccessModelInput) error {
	model := input.ModelName
	access := input.OldAccess

	uuid, err := c.ResolveModelUUID(model)
	if err != nil {
		return err
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

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
	id := strings.Split(input.Model, ":")
	model := id[0]

	uuid, err := c.ResolveModelUUID(model)
	if err != nil {
		return err
	}

	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	for _, user := range input.Revoke {
		err := client.RevokeModel(user, "read", uuid)
		if err != nil {
			return err
		}
	}

	return nil
}
