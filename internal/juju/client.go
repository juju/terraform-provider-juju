// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/core/model"
	"github.com/juju/terraform-provider-juju/internal/juju/modelcache"
)

const (
	PrefixCloud         = "cloud-"
	PrefixModel         = "model-"
	PrefixCharm         = "charm-"
	PrefixUser          = "user-"
	PrefixMachine       = "machine-"
	PrefixApplication   = "application-"
	PrefixStorage       = "storage-"
	UnspecifiedRevision = -1
	connectionTimeout   = 30 * time.Second
)

type ControllerConfiguration struct {
	ControllerAddresses []string
	Username            string
	Password            string
	CACert              string
	ClientID            string
	ClientSecret        string
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
	Secrets      secretsClient
}

type sharedClient struct {
	controllerConfig ControllerConfiguration

	modelCache modelcache.Cache

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context
}

// NewClient returns a client which can talk to the juju controller
// represented by controllerConfig. A context is required for logging in the
// terraform framework.
func NewClient(ctx context.Context, config ControllerConfiguration) (*Client, error) {
	if ctx == nil {
		return nil, errors.NotValidf("missing context")
	}
	sc := &sharedClient{
		controllerConfig: config,
		modelCache:       modelcache.NewModelCache(),
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
		Secrets:      *newSecretsClient(sc),
	}, nil
}

// GetConnection returns a juju connection for use creating juju
// api clients given the provided model name.
func (sc *sharedClient) GetConnection(modelName *string) (api.Connection, error) {
	var modelUUID string
	if modelName != nil {
		var err error
		modelUUID, err = sc.ModelUUID(*modelName)
		if err != nil {
			return nil, err
		}
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
		ClientID:            sc.controllerConfig.ClientID,
		ClientSecret:        sc.controllerConfig.ClientSecret,
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

func (sc *sharedClient) ModelUUID(modelName string) (string, error) {
	modelLookup := modelcache.NewModelLookup(modelName)
	if model, err := sc.modelCache.Lookup(modelLookup); err == nil {
		return model.UUID, nil
	}
	if err := sc.fillModelCache(); err != nil {
		return "", err
	}
	model, err := sc.modelCache.Lookup(modelLookup)
	if err != nil {
		return "", err
	}
	return model.UUID, nil
}

// fillModelCache checks with the juju controller for all
// models and puts the relevant data in the model info cache.
// Callers are expected to hold the modelUUIDmu lock.
func (sc *sharedClient) fillModelCache() error {
	conn, err := sc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := modelmanager.NewClient(conn)

	// Calling ListModelSummaries because other Model endpoints require
	// the UUID, here we're trying to get the model UUID for other calls.
	modelSummaries, err := client.ListModelSummaries(conn.AuthTag().Id(), false)
	if err != nil {
		return err
	}
	sc.modelCache.FillCache(modelSummaries)
	return nil
}

func (sc *sharedClient) ModelType(modelName string) (model.ModelType, error) {
	model, err := sc.modelCache.Lookup(modelcache.NewModelLookup(modelName))
	if err != nil {
		return "", err
	}
	return model.ModelType, nil
}

func (sc *sharedClient) RemoveModel(modelUUID string) {
	sc.modelCache.RemoveModel(modelUUID)
}

func (sc *sharedClient) AddModel(modelName, modelOwner, modelUUID string, modelType model.ModelType) {
	sc.modelCache.AddModel(modelName, modelOwner, modelUUID, modelType)
}

// module names for logging
// @module=juju.<subsystem>
// e.g.:
//
//	@module=juju.client
const LogJujuClient = "client"

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

func (sc *sharedClient) JujuLogger() *jujuLoggerShim {
	return &jujuLoggerShim{sc: sc}
}

// A shim to translate the juju/loggo package Errorf into
// the tflog SubsystemError. Used by apiclient.NewClient.
type jujuLoggerShim struct {
	sc *sharedClient
}

func (j jujuLoggerShim) Errorf(msg string, in ...interface{}) {
	stringInt := make(map[string]interface{}, len(in)+1)
	stringInt["error"] = msg
	for i, v := range in {
		stringInt[strconv.Itoa(i)] = v
	}
	tflog.SubsystemError(j.sc.subCtx, LogJujuClient, "juju api logging", map[string]interface{}{"error": msg})
}
