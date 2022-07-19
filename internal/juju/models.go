package juju

import (
	"fmt"
	"github.com/juju/juju/api"
	"log"
	"strings"
	"time"

	"github.com/juju/juju/api/client/modelconfig"

	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/controller/controller"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type modelsClient struct {
	ConnectionFactory
	store          jujuclient.ClientStore // TODO: This is currently not being used, but it may be needed in future so it is being retained for now
	controllerName string
}

func newModelsClient(cf ConnectionFactory, store jujuclient.ClientStore, controllerName string) *modelsClient {
	return &modelsClient{
		ConnectionFactory: cf,
		store:             store,
		controllerName:    controllerName,
	}
}

func (c *modelsClient) getControllerNameByUUID(conn api.Connection, uuid string) (*string, error) {
	client := controller.NewClient(conn)
	defer client.Close()

	controllerConfig, err := client.ControllerConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot find controller name from uuid: %s", uuid)
	}
	controllerName := controllerConfig.ControllerName()

	return &controllerName, nil
}

func (c *modelsClient) getCurrentUser(conn api.Connection) string {
	return strings.TrimPrefix(conn.AuthTag().String(), PrefixUser)
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

	currentUser := c.getCurrentUser(conn)
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

	log.Printf("[DEBUG] Reading model: %s, %+v", name, modelInfo)

	return modelInfo, nil
}

// ResolveModelUUID retrieves a model's using its name
func (c *modelsClient) ResolveModelUUID(name string) (string, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return "", err
	}

	currentUser := c.getCurrentUser(conn)
	client := modelmanager.NewClient(conn)
	defer client.Close()

	modelUUID, err := c.resolveModelUUIDWithClient(*client, name, currentUser)
	if err != nil {
		return "", nil
	}

	return modelUUID, nil
}

func (c *modelsClient) CreateModel(name string, cloudList []interface{}, cloudConfig map[string]interface{}) (*base.ModelInfo, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	currentUser := strings.TrimPrefix(conn.AuthTag().String(), PrefixUser)

	client := modelmanager.NewClient(conn)
	defer client.Close()

	cloudCredential := names.CloudCredentialTag{}

	//var controllerName string
	var cloudName string
	var cloudRegion string

	for _, cloud := range cloudList {
		cloudMap := cloud.(map[string]interface{})
		cloudName = cloudMap["name"].(string)
		cloudRegion = cloudMap["region"].(string)
	}

	modelInfo, err := client.CreateModel(name, currentUser, cloudName, cloudRegion, cloudCredential, cloudConfig)
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
	controllerName, err := c.getControllerNameByUUID(modelmanagerConn, modelInfo.ControllerUUID)
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
