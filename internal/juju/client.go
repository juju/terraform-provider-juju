package juju

import (
	"time"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

const (
	PrefixCloud         = "cloud-"
	PrefixModel         = "model-"
	PrefixCharm         = "charm-"
	UnspecifiedRevision = -1
	connectionTimeout   = 30 * time.Second
)

type Configuration struct {
	ControllerAddresses []string
	Username            string
	Password            string
	CACert              string
}

type Client struct {
	Models       modelsClient
	Applications applicationsClient
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
		Models:       *newModelsClient(cf, store, controllerName),
		Applications: *newApplicationClient(cf),
	}, nil
}

func (cf *ConnectionFactory) GetConnection(model *string) (api.Connection, error) {
	modelUUID := ""
	if model != nil {
		modelUUID = *model
	}

	dialOptions := func(do *api.DialOpts) {
		//this is set as a const above, in case we need to use it elsewhere to manage connection timings
		do.Timeout = connectionTimeout
		//default is 2 seconds, as we are changing the overall timeout it makes sense to reduce this as well
		do.RetryDelay = 1 * time.Second
	}

	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: cf.config.ControllerAddresses,
		Username:            cf.config.Username,
		Password:            cf.config.Password,
		CACert:              cf.config.CACert,
		ModelUUID:           modelUUID,
	}, dialOptions)
	if err != nil {
		return nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, err
	}

	return conn, nil
}
