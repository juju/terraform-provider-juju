package juju

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type Model struct {
	Name string
	Type string
	UUID string
}

type modelsClient struct {
	conn           api.Connection
	store          jujuclient.ClientStore
	controllerName string
}

func newModelsClient(conn api.Connection, store jujuclient.ClientStore, controllerName string) *modelsClient {
	return &modelsClient{
		conn:           conn,
		store:          store,
		controllerName: controllerName,
	}
}

func (c *modelsClient) getControllerNameByUUID(uuid string) (*string, error) {
	controllers, err := c.store.AllControllers()
	if err != nil {
		return nil, err
	}

	for name, details := range controllers {
		if details.ControllerUUID == uuid {
			return &name, nil
		}
	}

	return nil, errors.New(fmt.Sprintf("cannot find controller name from uuid: %s", uuid))
}

// GetByName retrieves a model by name
func (c *modelsClient) GetByName(name string) (Model, error) {
	client := modelmanager.NewClient(c.conn)
	defer client.Close()

	modelDetails, err := c.store.ModelByName(c.controllerName, name)
	if err != nil {
		return Model{}, err
	}

	modelTag := names.NewModelTag(modelDetails.ModelUUID)

	results, err := client.ModelInfo([]names.ModelTag{
		modelTag,
	})
	if err != nil {
		return Model{}, err
	}
	if results[0].Error != nil {
		return Model{}, results[0].Error
	}

	modelInfo := results[0].Result

	log.Printf("[DEBUG] Reading model: %s, %+v", name, modelInfo)

	return Model{
		Name: modelInfo.Name,
		Type: modelInfo.Type,
		UUID: modelInfo.UUID,
	}, nil
}

func (c *modelsClient) Create(name string, controller string, cloudList []interface{}, cloudConfig map[string]interface{}) (*base.ModelInfo, error) {
	client := modelmanager.NewClient(c.conn)
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
		var err error
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

	return &modelInfo, nil
}

func (c *modelsClient) Read(uuid string) (*string, *params.ModelInfo, error) {
	client := modelmanager.NewClient(c.conn)
	defer client.Close()

	models, err := client.ModelInfo([]names.ModelTag{names.NewModelTag(uuid)})
	if err != nil {
		return nil, nil, err
	}

	if len(models) > 1 {
		return nil, nil, errors.New(fmt.Sprintf("more than one model returned for UUID: %s", uuid))
	}
	if len(models) < 1 {
		return nil, nil, errors.New(fmt.Sprintf("no model returned for UUID: %s", uuid))
	}

	modelInfo := models[0].Result
	controllerName, err := c.getControllerNameByUUID(modelInfo.ControllerUUID)
	if err != nil {
		return nil, nil, err
	}

	return controllerName, modelInfo, nil

}

func (c *modelsClient) Destroy(uuid string) error {
	client := modelmanager.NewClient(c.conn)
	defer client.Close()

	maxWait := 10 * time.Minute
	timeout := 30 * time.Minute

	tag := names.NewModelTag(uuid)

	destroyStorage := true
	forceDestroy := false

	err := client.DestroyModel(tag, &destroyStorage, &forceDestroy, &maxWait, timeout)
	if err != nil {
		return err
	}

	return nil
}
