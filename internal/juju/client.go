// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/connector"
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

type ControllerConfiguration struct {
	ControllerAddresses []string
	Username            string
	Password            string
	CACert              string
}

type Client struct {
	Applications applicationsClient
	Machines     machinesClient
	Credentials  credentialsClient
	Integrations integrationsClient
	Models       modelsClient
	Offers       offersClient
	SSHKeys      sshKeysClient
	Users        usersClient
}

type SharedClient interface {
	GetConnection(modelName *string) (api.Connection, error)

	Debugf(msg string, additionalFields ...map[string]interface{})
	Errorf(err error, msg string)
	Tracef(msg string, additionalFields ...map[string]interface{})
	Warnf(msg string, additionalFields ...map[string]interface{})
}

type sharedClient struct {
	controllerConfig ControllerConfiguration

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

func NewClient(ctx context.Context, config ControllerConfiguration) (*Client, error) {
	sc := &sharedClient{
		controllerConfig: config,
		subCtx:           tflog.NewSubsystem(ctx, LogJujuClient),
	}

	return &Client{
		Applications: *newApplicationClient(sc),
		Credentials:  *newCredentialsClient(sc),
		Integrations: *newIntegrationsClient(sc),
		Machines:     *newMachinesClient(sc),
		Models:       *newModelsClient(sc),
		Offers:       *newOffersClient(sc),
		SSHKeys:      *newSSHKeysClient(sc),
		Users:        *newUsersClient(sc),
	}, nil
}

func (sc *sharedClient) GetConnection(model *string) (api.Connection, error) {
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
		ControllerAddresses: sc.controllerConfig.ControllerAddresses,
		Username:            sc.controllerConfig.Username,
		Password:            sc.controllerConfig.Password,
		CACert:              sc.controllerConfig.CACert,
		ModelUUID:           modelUUID,
	}, dialOptions)
	if err != nil {
		return nil, err
	}

	conn, err := connr.Connect()
	if err != nil {
		sc.Errorf(err, "connection not established")
		return nil, err
	}
	return conn, nil
}

// module names for logging
// @module=juju.<subsystem>
// e.g.:
//
//	@module=juju.client
const LogJujuClient = "client"

// TODO hml 04-Aug-2023
// Investigate if the context from terraform can be passed in
// and used with tflog here, for now, use context.Background.

func (sc *sharedClient) Debugf(msg string, additionalFields ...map[string]interface{}) {
	//SubsystemTrace(subCtx, "my-subsystem", "hello, world", map[string]interface{}{"foo": 123})
	// Output:
	// {"@level":"trace","@message":"hello, world","@module":"provider.my-subsystem","foo":123}
	tflog.SubsystemDebug(sc.subCtx, LogJujuClient, msg, additionalFields...)
}

func (sc *sharedClient) Errorf(err error, msg string) {
	tflog.SubsystemError(sc.subCtx, LogJujuClient, msg, map[string]interface{}{"error": err})
}

func (sc *sharedClient) Tracef(msg string, additionalFields ...map[string]interface{}) {
	tflog.SubsystemTrace(sc.subCtx, LogJujuClient, msg, additionalFields...)
}

func (sc *sharedClient) Warnf(msg string, additionalFields ...map[string]interface{}) {
	tflog.SubsystemWarn(sc.subCtx, LogJujuClient, msg, additionalFields...)
}

func getCurrentJujuUser(conn api.Connection) string {
	return conn.AuthTag().Id()
}
