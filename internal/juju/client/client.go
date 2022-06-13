package client

import (
	"github.com/juju/juju/api"
	"github.com/juju/juju/cmd/modelcmd"
	"github.com/juju/juju/jujuclient"
)

type Client struct {
	Models modelClient
}

func New(conn api.Connection) *Client {

	var store jujuclient.ClientStore = modelcmd.QualifyingClientStore{
		ClientStore: jujuclient.NewFileClientStore(),
	}

	const controllerName = "overlord"

	return &Client{
		Models: *newModelClient(conn, store, controllerName),
	}
}
