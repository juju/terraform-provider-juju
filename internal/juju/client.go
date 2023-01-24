package juju

import (
	"time"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/connector"
	"github.com/rs/zerolog/log"
)

const (
	PrefixCloud         = "cloud-"
	PrefixModel         = "model-"
	PrefixCharm         = "charm-"
	PrefixUser          = "user-"
	PrefixMachine       = "machine-"
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
	Machines     machinesClient
	Integrations integrationsClient
	Offers       offersClient
}

type ConnectionFactory struct {
	config Configuration
}

func NewClient(config Configuration) (*Client, error) {
	cf := ConnectionFactory{
		config: config,
	}

	return &Client{
		Models:       *newModelsClient(cf),
		Applications: *newApplicationClient(cf),
		Integrations: *newIntegrationsClient(cf),
		Machines:     *newMachinesClient(cf),
		Offers:       *newOffersClient(cf),
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
		log.Error().Err(err).Msg("connection not established")
		return nil, err
	}
	return conn, nil
}
