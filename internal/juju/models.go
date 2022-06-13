package juju

import (
	"github.com/juju/errors"
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

type modelClient struct {
	conn           api.Connection
	store          jujuclient.ClientStore
	controllerName string
}

func newModelClient(conn api.Connection, store jujuclient.ClientStore, controllerName string) *modelClient {
	return &modelClient{
		conn:           conn,
		store:          store,
		controllerName: controllerName,
	}
}

// GetByName retrieves a model by name
func (c *modelClient) GetByName(name string) (Model, error) {
	client := modelmanager.NewClient(c.conn)
	defer client.Close()

	modelDetails, err := c.store.ModelByName(c.controllerName, name)
	if !errors.IsNotFound(err) {
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
