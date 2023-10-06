// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/core/model"
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

type jujuModel struct {
	uuid      string
	modelType model.ModelType
}

func (j jujuModel) String() string {
	return fmt.Sprintf("uuid(%s) type(%s)", j.uuid, j.modelType.String())
}

type SharedClient interface {
	AddModel(modelName, modelUUID string, modelType model.ModelType)
	GetConnection(modelName *string) (api.Connection, error)
	ModelType(modelName string) (model.ModelType, error)
	ModelUUID(modelName string) (string, error)
	RemoveModel(modelUUID string)

	Debugf(msg string, additionalFields ...map[string]interface{})
	Errorf(err error, msg string)
	Tracef(msg string, additionalFields ...map[string]interface{})
	Warnf(msg string, additionalFields ...map[string]interface{})
}

type sharedClient struct {
	controllerConfig ControllerConfiguration

	modelUUIDcache map[string]jujuModel
	modelUUIDmu    sync.Mutex

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
		modelUUIDcache:   make(map[string]jujuModel),
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
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	dataMap := make(map[string]interface{})
	// How to tell if logging level is Trace?
	for k, v := range sc.modelUUIDcache {
		dataMap[k] = v.String()
	}
	sc.Tracef(fmt.Sprintf("ModelUUID cache looking for %q", modelName), dataMap)
	if modelWithName, ok := sc.modelUUIDcache[modelName]; ok {
		sc.Tracef(fmt.Sprintf("Found uuid for %q in cache", modelName))
		return modelWithName.uuid, nil
	}
	if err := sc.fillModelCache(); err != nil {
		return "", err
	}
	if modelWithName, ok := sc.modelUUIDcache[modelName]; ok {
		sc.Tracef(fmt.Sprintf("Found uuid for %q in cache on 2nd attempt", modelName))
		return modelWithName.uuid, nil
	}
	return "", errors.NotFoundf("model %q", modelName)
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
	for _, modelSummary := range modelSummaries {
		modelWithName := jujuModel{
			uuid:      modelSummary.UUID,
			modelType: modelSummary.Type,
		}
		sc.modelUUIDcache[modelSummary.Name] = modelWithName
	}
	return nil
}

func (sc *sharedClient) ModelType(modelName string) (model.ModelType, error) {
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	if modelWithName, ok := sc.modelUUIDcache[modelName]; ok {
		return modelWithName.modelType, nil
	}

	return model.ModelType(""), errors.NotFoundf("type for model %q", modelName)
}

func (sc *sharedClient) RemoveModel(modelUUID string) {
	sc.modelUUIDmu.Lock()
	var modelName string
	for k, v := range sc.modelUUIDcache {
		if v.uuid == modelUUID {
			modelName = k
			break
		}
	}
	if modelName != "" {
		delete(sc.modelUUIDcache, modelName)
	}
	sc.modelUUIDmu.Unlock()
}

func (sc *sharedClient) AddModel(modelName, modelUUID string, modelType model.ModelType) {
	sc.modelUUIDmu.Lock()
	sc.modelUUIDcache[modelName] = jujuModel{
		uuid:      modelUUID,
		modelType: modelType,
	}
	sc.modelUUIDmu.Unlock()
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
