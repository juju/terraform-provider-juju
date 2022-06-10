package client

import (
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/connector"
)

type InternalClient struct {
	api.Connection
}

// NewClient Returns a new InternalClient with a connection object to the required controller
func NewClient(config connector.SimpleConfig) (*InternalClient, error) {
	connr, err := connector.NewSimple(config)
	if err != nil {
		return nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		return nil, err
	}

	internalClient := InternalClient{
		Connection: conn,
	}

	return &internalClient, nil
}
