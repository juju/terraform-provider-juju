package juju

import (
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/jujuclient"
	"github.com/juju/names/v4"
	"log"
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
