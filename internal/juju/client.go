// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"strconv"
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
	Jaas         jaasClient

	isJAAS func() bool
}

// IsJAAS returns a boolean to indicate whether the controller configured is a JAAS controller.
// JAAS controllers offer additional functionality for permission management.
func (c Client) IsJAAS() bool {
	return c.isJAAS()
}

type jujuModel struct {
	uuid      string
	modelType model.ModelType
}

func (j jujuModel) String() string {
	return fmt.Sprintf("uuid(%s) type(%s)", j.uuid, j.modelType.String())
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
	// Client ID and secret are only set when connecting to JAAS. Use this as a fallback
	// value if connecting to the controller fails.
	defaultJAASCheck := false
	if config.ClientID != "" && config.ClientSecret != "" {
		defaultJAASCheck = true
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
		Jaas:         *newJaasClient(sc),
		isJAAS:       func() bool { return sc.IsJAAS(defaultJAASCheck) },
	}, nil
}

var checkJAASOnce sync.Once
var isJAAS bool

// IsJAAS checks if the controller is a JAAS controller.
// It does this by checking whether it offers the "JIMM" facade which
// will only ever be offered by JAAS. The method accepts a default value
// and doesn't return an error because callers are not expected to fail if
// they can't determine whether they are connecting to JAAS.
//
// IsJAAS uses a synchronisation object to only perform the check once and return the same result.
func (sc *sharedClient) IsJAAS(defaultVal bool) bool {
	checkJAASOnce.Do(func() {
		conn, err := sc.GetConnection(nil)
		if err != nil {
			isJAAS = defaultVal
			return
		}
		defer conn.Close()
		isJAAS = conn.BestFacadeVersion("JIMM") != 0
	})
	return isJAAS
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
