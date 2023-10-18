// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"time"

	"github.com/juju/errors"
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
	toReturn := "model %q was not found"
	if me.name != "" {
		return fmt.Sprintf(toReturn, me.name)
	}
	return fmt.Sprintf(toReturn, me.uuid)
}

type modelsClient struct {
	SharedClient
}

type GrantModelInput struct {
	User      string
	Access    string
	ModelName string
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

type UpdateModelInput struct {
	Name        string
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
	ModelName string
	Revoke    []string
	Access    string
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
		return resp, err
	}

	resp.Cloud = modelInfo.Cloud
	resp.CloudRegion = modelInfo.CloudRegion
	resp.CloudCredentialName = names.NewCloudCredentialTag(modelInfo.CloudCredential).Name()
	resp.Type = modelInfo.Type.String()
	resp.UUID = modelInfo.UUID

	// Add the model to the client cache of jujuModel
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
		return nil, errors.Wrap(err, &modelNotFoundError{uuid: name})
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
		return nil, &modelNotFoundError{uuid: modelUUIDTag.Id()}
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
	conn, err := c.GetConnection(&input.Name)
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

	modelUUID, err := c.ModelUUID(input.ModelName)
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
	model := input.ModelName
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

	uuid, err := c.ModelUUID(input.ModelName)
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
