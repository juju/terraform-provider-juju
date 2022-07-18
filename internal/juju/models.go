package juju

import (
	"fmt"
	"log"
	"time"

	"github.com/juju/juju/api/client/modelconfig"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type modelsClient struct {
	ConnectionFactory
	store          jujuclient.ClientStore
	controllerName string
}

func newModelsClient(cf ConnectionFactory, store jujuclient.ClientStore, controllerName string) *modelsClient {
	return &modelsClient{
		ConnectionFactory: cf,
		store:             store,
		controllerName:    controllerName,
	}
}

func (c *modelsClient) getControllerNameByUUID(uuid string) (*string, error) {
	// TODO: find alternative for this so dependency on the client store can be removed
	controllers, err := c.store.AllControllers()
	if err != nil {
		return nil, err
	}

	for name, details := range controllers {
		if details.ControllerUUID == uuid {
			return &name, nil
		}
	}

	return nil, fmt.Errorf("cannot find controller name from uuid: %s", uuid)
}

// GetModelByName retrieves a model by name
func (c *modelsClient) GetModelByName(name string) (*params.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	// TODO: remove dependency on store, see ListModelSummaries call instead
	modelDetails, err := c.store.ModelByName(c.controllerName, name)
	if err != nil {
		return nil, err
	}

	modelTag := names.NewModelTag(modelDetails.ModelUUID)

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

	log.Printf("[DEBUG] Reading model: %s, %+v", name, modelInfo)

	return modelInfo, nil
}

// ResolveModelUUID retrieves a model's using its name
func (c *modelsClient) ResolveModelUUID(name string) (string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return "", err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	modelDetails, err := c.store.ModelByName(c.controllerName, name)
	if err != nil {
		return "", err
	}

	return modelDetails.ModelUUID, nil
}

func (c *modelsClient) CreateModel(name string, controller string, cloudList []interface{}, cloudConfig map[string]interface{}) (*base.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	cloudCredential := names.CloudCredentialTag{}

	var controllerName string
	var cloudName string
	var cloudRegion string

	for _, cloud := range cloudList {
		cloudMap := cloud.(map[string]interface{})
		cloudName = cloudMap["name"].(string)
		cloudRegion = cloudMap["region"].(string)
	}

	if controller == "" {
		controllerName, err = c.store.CurrentController()
		if err != nil {
			return nil, err
		}
	} else {
		controllerName = controller
	}

	accountDetails, err := c.store.AccountDetails(controllerName)
	if err != nil {
		return nil, err
	}

	modelInfo, err := client.CreateModel(name, accountDetails.User, cloudName, cloudRegion, cloudCredential, cloudConfig)
	if err != nil {
		return nil, err
	}

	// TODO: integrate more gracefully
	// This updates the client filestore with the model details which is required for tests to pass
	err = c.store.UpdateModel(controllerName, name, jujuclient.ModelDetails{
		ModelUUID: modelInfo.UUID,
		ModelType: modelInfo.Type,
	})
	if err != nil {
		return nil, err
	}

	return &modelInfo, nil
}

func (c *modelsClient) ReadModel(uuid string) (*string, *params.ModelInfo, map[string]interface{}, error) {
	modelmanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, nil, nil, err
	}

	modelconfigConn, err := c.GetConnection(&uuid)
	if err != nil {
		return nil, nil, nil, err
	}

	modelmanagerClient := modelmanager.NewClient(modelmanagerConn)
	defer modelmanagerClient.Close()

	modelconfigClient := modelconfig.NewClient(modelconfigConn)
	defer modelconfigClient.Close()

	models, err := modelmanagerClient.ModelInfo([]names.ModelTag{names.NewModelTag(uuid)})
	if err != nil {
		return nil, nil, nil, err
	}

	if len(models) > 1 {
		return nil, nil, nil, fmt.Errorf("more than one model returned for UUID: %s", uuid)
	}
	if len(models) < 1 {
		return nil, nil, nil, fmt.Errorf("no model returned for UUID: %s", uuid)
	}

	modelInfo := models[0].Result
	controllerName, err := c.getControllerNameByUUID(modelInfo.ControllerUUID)
	if err != nil {
		return nil, nil, nil, err
	}

	modelConfig, err := modelconfigClient.ModelGet()
	if err != nil {
		return nil, nil, nil, err
	}

	return controllerName, modelInfo, modelConfig, nil
}

func (c *modelsClient) UpdateModel(uuid string, config map[string]interface{}, unset []string) error {
	conn, err := c.GetConnection(&uuid)
	if err != nil {
		return err
	}

	client := modelconfig.NewClient(conn)
	defer client.Close()

	err = client.ModelSet(config)
	if err != nil {
		return err
	}

	err = client.ModelUnset(unset...)
	if err != nil {
		return err
	}

	return nil
}

func (c *modelsClient) DestroyModel(uuid string) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := modelmanager.NewClient(conn)
	defer client.Close()

	maxWait := 10 * time.Minute
	timeout := 30 * time.Minute

	tag := names.NewModelTag(uuid)

	destroyStorage := true
	forceDestroy := false

	err = client.DestroyModel(tag, &destroyStorage, &forceDestroy, &maxWait, timeout)
	if err != nil {
		return err
	}

	return nil
}
