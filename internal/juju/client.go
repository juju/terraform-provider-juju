package juju

import (
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
	Models modelClient
}

func NewClient(config Configuration) (*Client, error) {
	connr, err := connector.NewSimple(connector.SimpleConfig{
		ControllerAddresses: config.ControllerAddresses,
		Username:            config.Username,
		Password:            config.Password,
		CACert:              config.CACert,
	})
	if err != nil {
		return nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, err
	}

	var store jujuclient.ClientStore = modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}

	// TODO: determine how to obtain this
	const controllerName = "overlord"

	return &Client{
		Models: *newModelClient(conn, store, controllerName),
	}, nil
}
