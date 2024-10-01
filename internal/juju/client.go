// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	jaasApi "github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/core/model"
	"github.com/juju/names/v5"
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
	name      string
	modelType model.ModelType
}

func (j jujuModel) String() string {
	return fmt.Sprintf("uuid(%s) type(%s)", j.name, j.modelType.String())
}

type sharedClient struct {
	controllerConfig ControllerConfiguration

	modelUUIDcache map[string]jujuModel
	modelUUIDmu    sync.Mutex

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context

	checkJAASOnce sync.Once
	isJAAS        bool
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

// IsJAAS checks if the controller is a JAAS controller.
// It does this by checking whether a JIMM specific call can be made.
// The method accepts a default value and doesn't return an error
// because callers are not expected to fail if they can't determine
// whether they are connecting to JAAS.
//
// IsJAAS uses a synchronisation object to only perform the check once and return the same result.
func (sc *sharedClient) IsJAAS(defaultVal bool) bool {
	sc.checkJAASOnce.Do(func() {
		sc.isJAAS = defaultVal
		conn, err := sc.GetConnection(nil)
		if err != nil {
			return
		}
		defer conn.Close()
		jc := jaasApi.NewClient(conn)
		_, err = jc.ListControllers()
		if err == nil {
			sc.isJAAS = true
			return
		}
	})
	return sc.isJAAS
}

// GetConnection returns a juju connection for use creating juju
// api clients given the provided model uuid, name, or neither.
// Allowing a model name is a fallback behavior until the name
// used by most resources has been removed in favor of the UUID.
func (sc *sharedClient) GetConnection(modelIdentifier *string) (api.Connection, error) {
	var modelUUID string
	if modelIdentifier != nil {
		var err error
		modelUUID, err = sc.ModelUUID(*modelIdentifier)
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

// ModelUUID returns the model uuid for the provided modelIdentifier.
// The modelIdentifier can be a model name or model uuid. If a name
// is provided, first search the modelUUIDCache for the uuid. If it's
// not found, fill the model cache and try again. If the modelIdentifier
// is a uuid, return that without verification.
func (sc *sharedClient) ModelUUID(modelIdentifier string) (string, error) {
	if names.IsValidModel(modelIdentifier) {
		return modelIdentifier, nil
	}
	modelName := modelIdentifier
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	dataMap := make(map[string]interface{})
	// How to tell if logging level is Trace?
	for k, v := range sc.modelUUIDcache {
		dataMap[k] = v.String()
	}
	sc.Tracef(fmt.Sprintf("ModelUUID cache looking for %q", modelName), dataMap)
	for uuid, m := range sc.modelUUIDcache {
		if m.name == modelName {
			sc.Tracef(fmt.Sprintf("Found uuid for %q in cache", modelName))
			return uuid, nil
		}
	}
	if err := sc.fillModelCache(); err != nil {
		return "", err
	}
	for uuid, m := range sc.modelUUIDcache {
		if m.name == modelName {
			sc.Tracef(fmt.Sprintf("Found uuid for %q in cache on 2nd attempt", modelName))
			return uuid, nil
		}
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
		modelWithUUID := jujuModel{
			name:      modelSummary.Name,
			modelType: modelSummary.Type,
		}
		sc.modelUUIDcache[modelSummary.UUID] = modelWithUUID
	}
	return nil
}

func (sc *sharedClient) ModelName(modelIdentifier string) (string, error) {
	if !names.IsValidModel(modelIdentifier) {
		return modelIdentifier, nil
	}
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	jModel, ok := sc.modelUUIDcache[modelIdentifier]
	if ok {
		return jModel.name, nil
	}
	if err := sc.fillModelCache(); err != nil {
		return "", err
	}
	jModel, ok = sc.modelUUIDcache[modelIdentifier]
	var err error
	if !ok {
		err = fmt.Errorf("unable to find model name for %q", modelIdentifier)
	}
	return jModel.name, err
}

func (sc *sharedClient) ModelType(modelIdentifier string) (model.ModelType, error) {
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	if names.IsValidModel(modelIdentifier) {
		if modelWithUUID, ok := sc.modelUUIDcache[modelIdentifier]; ok {
			return modelWithUUID.modelType, nil
		}
	}

	for _, m := range sc.modelUUIDcache {
		if m.name == modelIdentifier {
			return m.modelType, nil
		}
	}

	return model.ModelType(""), errors.NotFoundf("type for model %q", modelIdentifier)
}

func (sc *sharedClient) RemoveModel(modelUUID string) {
	sc.modelUUIDmu.Lock()
	delete(sc.modelUUIDcache, modelUUID)
	sc.modelUUIDmu.Unlock()
}

func (sc *sharedClient) AddModel(modelName, modelUUID string, modelType model.ModelType) {
	sc.modelUUIDmu.Lock()
	sc.modelUUIDcache[modelUUID] = jujuModel{
		name:      modelName,
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
