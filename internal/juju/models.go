package juju

import (
	"github.com/juju/juju/api"
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
