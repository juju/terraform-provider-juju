// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jaasApi "github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/api/client/modelmanager"
	"github.com/juju/juju/api/connector"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"
	"github.com/juju/utils/cache"
)

const (
	PrefixCloud                          = "cloud-"
	PrefixModel                          = "model-"
	PrefixCharm                          = "charm-"
	PrefixUser                           = "user-"
	PrefixMachine                        = "machine-"
	PrefixApplication                    = "application-"
	PrefixStorage                        = "storage-"
	UnspecifiedRevision                  = -1
	customTimeoutKey                     = "JUJU_CONNECTION_TIMEOUT"
	waitForResourcesKey                  = "JUJU_WAIT_FOR_RESOURCES"
	connectionTimeout                    = 30 * time.Second
	serviceAccountSuffix                 = "@serviceaccount"
	defaultModelStatusCacheInterval      = 5 * time.Second
	defaultModelStatusCacheRetryInterval = defaultModelStatusCacheInterval / 2
	ReadModelDefaultInterval             = defaultModelStatusCacheInterval / 2
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
	Machines     *machinesClient
	Clouds       kubernetesCloudsClient
	Credentials  credentialsClient
	Integrations integrationsClient
	Models       modelsClient
	Offers       offersClient
	SSHKeys      sshKeysClient
	Users        usersClient
	Secrets      secretsClient
	Jaas         jaasClient
	Annotations  annotationsClient

	isJAAS   func() bool
	username string
}

// ConnectionRefusedError is a global variable that can be used to check
// if an error is a connectionRefusedError. This is useful for retry logic
// where you want to retry on connection refused errors.
var ConnectionRefusedError = errors.ConstError("connection refused")

// IsJAAS returns a boolean to indicate whether the controller configured is a JAAS controller.
// JAAS controllers offer additional functionality for permission management.
func (c Client) IsJAAS() bool {
	return c.isJAAS()
}

// Username returns the username specified in the Juju provider or, if specified, the
// service account username.
func (c Client) Username() string {
	return c.username
}

type jujuModel struct {
	name      string
	owner     string
	modelType model.ModelType
}

func (j jujuModel) String() string {
	return fmt.Sprintf("uuid(%s) type(%s)", j.name, j.modelType.String())
}

type sharedClient struct {
	controllerConfig ControllerConfiguration
	waitForResources bool

	modelCacheOnce sync.Once
	modelUUIDcache map[string]jujuModel
	modelUUIDmu    sync.Mutex

	modelStatusCache *cache.Cache

	// subCtx is the context created with the new tflog subsystem for applications.
	subCtx context.Context

	checkJAASOnce sync.Once
	isJAAS        bool
}

// NewClient returns a client which can talk to the juju controller
// represented by controllerConfig. A context is required for logging in the
// terraform framework.
func NewClient(ctx context.Context, config ControllerConfiguration, waitForResources bool) (*Client, error) {
	if ctx == nil {
		return nil, errors.NotValidf("missing context")
	}
	sc := &sharedClient{
		controllerConfig: config,
		waitForResources: waitForResources,
		modelUUIDcache:   make(map[string]jujuModel),
		modelStatusCache: cache.New(defaultModelStatusCacheInterval),
		subCtx:           tflog.NewSubsystem(ctx, LogJujuClient),
	}
	// Client ID and secret are only set when connecting to JAAS. Use this as a fallback
	// value if connecting to the controller fails.
	defaultJAASCheck := false
	if config.ClientID != "" && config.ClientSecret != "" {
		defaultJAASCheck = true
	}

	user := config.Username
	if config.ClientID != "" && !strings.HasSuffix(config.ClientID, serviceAccountSuffix) {
		user = fmt.Sprintf("%s%s", config.ClientID, serviceAccountSuffix)
	}

	return &Client{
		Applications: *newApplicationClient(sc),
		Clouds:       *newKubernetesCloudsClient(sc),
		Credentials:  *newCredentialsClient(sc),
		Integrations: *newIntegrationsClient(sc),
		Machines:     newMachinesClient(sc),
		Models:       *newModelsClient(sc),
		Offers:       *newOffersClient(sc),
		SSHKeys:      *newSSHKeysClient(sc),
		Users:        *newUsersClient(sc),
		Secrets:      *newSecretsClient(sc),
		Jaas:         *newJaasClient(sc),
		Annotations:  *newAnnotationsClient(sc),
		isJAAS:       func() bool { return sc.IsJAAS(defaultJAASCheck) },
		username:     user,
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

func getConnectionTimeout() time.Duration {
	if timeout, ok := os.LookupEnv(customTimeoutKey); ok {
		if t, err := strconv.Atoi(timeout); err == nil && t > 0 {
			return time.Duration(t) * time.Second
		}
		tflog.Warn(context.Background(), "Invalid JUJU_CONNECTION_TIMEOUT value, using default", map[string]interface{}{
			"JUJU_CONNECTION_TIMEOUT": timeout,
			"default":                 connectionTimeout,
		})
	}
	return connectionTimeout
}

// WaitForResource returns a bool indicating whether the client
// should wait for resources to be available/destroyed before proceeding.
func (sc *sharedClient) WaitForResource() bool {
	return sc.waitForResources
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
		do.Timeout = getConnectionTimeout()
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

// initializeModelCache is a helper function to ensure that the model cache is filled at
// least once. It should be called before accessing the model cache to ensure that
// the cache is populated with model data.
func (sc *sharedClient) initializeModelCache() {
	sc.modelCacheOnce.Do(func() {
		if err := sc.fillModelCache(); err != nil {
			// Log the error and continue
			sc.Errorf(err, "failed to do initial fill of the model cache")
		}
	})
}

// ModelOwnerAndName returns the owner and name of the model identified by its UUID.
func (sc *sharedClient) ModelOwnerAndName(modelUUID string) (owner, name string, err error) {
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()

	sc.initializeModelCache()
	modelInfo, ok := sc.modelUUIDcache[modelUUID]
	if !ok {
		return "", "", errors.NotFoundf("model %q", modelUUID)
	}
	return modelInfo.owner, modelInfo.name, nil
}

// ModelUUID returns the model uuid for the provided modelIdentifier.
// The modelIdentifier can be a model name or model uuid. If the modelIdentifier
// is a uuid, return that without verification. If a name is provided, it will
// search the modelUUIDCache for the uuid.
func (sc *sharedClient) ModelUUID(modelIdentifier string) (string, error) {
	if names.IsValidModel(modelIdentifier) {
		return modelIdentifier, nil
	}
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()

	sc.initializeModelCache()
	modelName := modelIdentifier
	sc.Tracef(fmt.Sprintf("ModelUUID cache looking for %q", modelName))
	for uuid, m := range sc.modelUUIDcache {
		if m.name == modelName {
			sc.Tracef(fmt.Sprintf("Found uuid for %q in cache", modelName))
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
			owner:     modelSummary.Owner,
		}
		sc.modelUUIDcache[modelSummary.UUID] = modelWithUUID
	}
	return nil
}

// ModelType returns the model type for the provided modelUUID from
// the cache of model data.
func (sc *sharedClient) ModelType(modelUUID string) (model.ModelType, error) {
	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	sc.initializeModelCache()
	if !names.IsValidModel(modelUUID) {
		return "", errors.NotValidf("modelUUID %q is not a valid model UUID", modelUUID)
	}
	if modelWithUUID, ok := sc.modelUUIDcache[modelUUID]; ok {
		return modelWithUUID.modelType, nil
	}
	return "", errors.NotFoundf("type for model %q", modelUUID)
}

// RemoveModel deletes the model with the given UUID from the cache of
// model data.
func (sc *sharedClient) RemoveModel(modelUUID string) {
	sc.modelUUIDmu.Lock()
	delete(sc.modelUUIDcache, modelUUID)
	sc.modelUUIDmu.Unlock()
}

// AddModel adds a model to the cache of model data. If any of the three required
// pieces of data are empty, nothing is added to the cache of model data. If the UUID
// already exists in the cache, do nothing.
func (sc *sharedClient) AddModel(modelName, modelOwner, modelUUID string, modelType model.ModelType) {
	if modelName == "" || !names.IsValidModel(modelUUID) || modelType.String() == "" {
		sc.Tracef("Missing data, failed to add to the cache.", map[string]interface{}{
			"modelName": modelName, "modelUUID": modelUUID, "modelType": modelType.String()})
		return
	}

	sc.modelUUIDmu.Lock()
	defer sc.modelUUIDmu.Unlock()
	if m, ok := sc.modelUUIDcache[modelUUID]; ok {
		sc.Warnf("Attempting to add an existing model to the cache.", map[string]interface{}{
			"existing model in cache": m, "new modelName": modelName, "new modelUUID": modelUUID,
			"new modelType": modelType.String()})
		return
	}
	sc.modelUUIDcache[modelUUID] = jujuModel{
		name:      modelName,
		owner:     modelOwner,
		modelType: modelType,
	}
}

func (sc *sharedClient) getModelStatusFunc(uuid string, conn api.Connection) func() (interface{}, error) {
	return func() (interface{}, error) {
		var err error
		if conn == nil {
			conn, err = sc.GetConnection(&uuid)
			if err != nil {
				return nil, err
			}
		}

		client := apiclient.NewClient(conn, sc.JujuLogger())
		status, err := client.Status(nil)
		if err != nil {
			return nil, err
		}

		return status, nil
	}
}

// ModelStatus returns the status of the model identified by its UUID.
func (sc *sharedClient) ModelStatus(modelUUID string, conn api.Connection) (*params.FullStatus, error) {
	status, err := sc.modelStatusCache.Get(modelUUID, sc.getModelStatusFunc(modelUUID, conn))
	if err != nil {
		return nil, err
	}

	modelStatus, ok := status.(*params.FullStatus)
	if !ok {
		return nil, errors.Errorf("model status cache error: expected %T, got %T", modelStatus, status)
	}

	return modelStatus, nil
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
