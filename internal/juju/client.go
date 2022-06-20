package juju

import (
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

type Configuration struct {
	ControllerAddresses []string
	Username            string
	Password            string
	CACert              string
}

type Client struct {
	Models modelsClient
}

type ConnectionFactory struct {
	config Configuration
}

func NewClient(config Configuration) (*Client, error) {
	cf := ConnectionFactory{
		config: config,
	}

	var store jujuclient.ClientStore = modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}

	// TODO: should the controller be part of the provider configuration?
	controllerName, err := store.CurrentController()
	if err != nil {
		return nil, err
	}

	return &Client{
		Models: *newModelsClient(cf, store, controllerName),
	}, nil
}

func (cf *ConnectionFactory) GetConnection(model *string) (api.Connection, error) {
	modelUUID := ""
	if model != nil {
		modelUUID = *model
	}

	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: cf.config.ControllerAddresses,
		Username:            cf.config.Username,
		Password:            cf.config.Password,
		CACert:              cf.config.CACert,
		ModelUUID:           modelUUID,
	})
	if err != nil {
		return nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, err
	}

	return conn, nil
}
