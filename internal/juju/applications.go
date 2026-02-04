// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Package juju is a facade to make interacting with Juju clients simpler. It also acts as an insulating layer
// protecting the provider package from upstream changes.
// The long-term intention is for this package to be removed. Eventually, it would be nice for this package to
// be replaced with more granular clients in Juju itself. Note that much of this code is duplicated from Juju CLI
// commands.
package juju

import (
	"bytes"
	"context"
	stderrors "errors"
	"fmt"
	"math"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/juju/clock"
	"github.com/juju/collections/set"
	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apiclient "github.com/juju/juju/api/client/client"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	apiresources "github.com/juju/juju/api/client/resources"
	apispaces "github.com/juju/juju/api/client/spaces"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/cmd/juju/application/utils"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/network"
	"github.com/juju/juju/core/semversion"
	jujustorage "github.com/juju/juju/core/storage"
	jujuversion "github.com/juju/juju/core/version"
	"github.com/juju/juju/domain/deployment/charm"
	charmresources "github.com/juju/juju/domain/deployment/charm/resource"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v6"
	"github.com/juju/retry"
	goyaml "gopkg.in/yaml.v2"
)

// NewApplicationNotFoundError returns a new error indicating that the
// application was not found.
func NewApplicationNotFoundError(appName string) error {
	return jujuerrors.WithType(jujuerrors.Errorf("application %s not found", appName), ApplicationNotFoundError)
}

// ApplicationNotFoundError is an error that indicates that the application
// was not found when contacting the Juju API.
var ApplicationNotFoundError = jujuerrors.ConstError("application-not-found")

// NewStorageNotFoundError returns a new error indicating that the
// storage was not found.
func NewStorageNotFoundError(applicationName string) error {
	return jujuerrors.WithType(jujuerrors.Errorf("storage not found for application %s", applicationName), StorageNotFoundError)
}

// StorageNotFoundError is an error that indicates that the storage was not found.
var StorageNotFoundError = jujuerrors.ConstError("storage-not-found")

// RetryReadError is an error that indicates that a read operation
// should be retried. This is used to handle transient errors
// that may occur when reading application data from the Juju API.
var RetryReadError = jujuerrors.ConstError("retry-read-error")

// NewRetryReadError returns a new retry error with the specified message.
func NewRetryReadError(msg string) error {
	return jujuerrors.WithType(jujuerrors.Errorf("retrying: %s", msg), RetryReadError)
}

type ApplicationPartiallyCreatedError struct {
	AppName string
}

func (e ApplicationPartiallyCreatedError) Error() string {
	return "application " + e.AppName + " was partially created"
}

func newApplicationPartiallyCreatedError(appName string) error {
	return ApplicationPartiallyCreatedError{AppName: appName}
}

type applicationsClient struct {
	SharedClient
	controllerVersion semversion.Number

	getApplicationAPIClient func(base.APICallCloser) ApplicationAPIClient
	getClientAPIClient      func(api.Connection) ClientAPIClient
	getModelConfigAPIClient func(api.Connection) ModelConfigAPIClient
	getResourceAPIClient    func(connection api.Connection) (ResourceAPIClient, error)
	getCharmClient          func(api.Connection) *charmsClient
}

func newApplicationClient(sc SharedClient) *applicationsClient {
	return &applicationsClient{
		SharedClient: sc,
		getApplicationAPIClient: func(closer base.APICallCloser) ApplicationAPIClient {
			return apiapplication.NewClient(closer)
		},
		getClientAPIClient: func(conn api.Connection) ClientAPIClient {
			return apiclient.NewClient(conn, sc.JujuLogger())
		},
		getModelConfigAPIClient: func(conn api.Connection) ModelConfigAPIClient {
			return apimodelconfig.NewClient(conn)
		},
		getResourceAPIClient: func(conn api.Connection) (ResourceAPIClient, error) {
			return apiresources.NewClient(conn)
		},
		getCharmClient: func(conn api.Connection) *charmsClient {
			return newCharmsClient(conn)
		},
	}
}

// ConfigEntry is an auxiliary struct to keep information about
// juju application config entries. Specially, we want to know
// if they have the default value.
type ConfigEntry struct {
	Value     interface{}
	IsDefault bool
}

// EqualConfigEntries compare two juju configuration entries.
// If both entries share the same type, otherwise they are
// considered to be different.
func EqualConfigEntries(a interface{}, b interface{}) bool {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	return a == b
}

func (ce *ConfigEntry) String() string {
	return ConfigEntryToString(ce.Value)
}

// ConfigEntryToString returns the string representation based on
// the current value.
func ConfigEntryToString(input interface{}) string {
	switch t := input.(type) {
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	case float64:
		return strconv.FormatFloat(t, 'f', 0, 64)
	default:
		return input.(string)
	}
}

type CreateApplicationInput struct {
	ApplicationName    string
	ModelUUID          string
	CharmName          string
	CharmChannel       string
	CharmBase          string
	CharmSeries        string
	CharmRevision      int
	Units              int
	Trust              bool
	Expose             map[string]interface{}
	Config             map[string]string
	Placement          string
	Machines           []string
	Constraints        constraints.Value
	EndpointBindings   map[string]string
	Resources          map[string]CharmResource
	StorageConstraints map[string]jujustorage.Directive
}

// validateAndTransform returns transformedCreateApplicationInput which
// validated and in the proper format for both the new and legacy deployment
// methods. Select input is not transformed due to differences in the
// 2 deployment methods, such as config.
func (input CreateApplicationInput) validateAndTransform(ctx context.Context, conn api.Connection) (parsed transformedCreateApplicationInput, err error) {
	parsed.charmChannel = input.CharmChannel
	parsed.charmName = input.CharmName
	parsed.charmRevision = input.CharmRevision
	parsed.constraints = input.Constraints
	parsed.config = input.Config
	parsed.expose = input.Expose
	parsed.trust = input.Trust
	parsed.units = input.Units
	parsed.resources = input.Resources
	parsed.storage = input.StorageConstraints

	appName := input.ApplicationName
	if appName == "" {
		appName = input.CharmName
	}
	if err = names.ValidateApplicationName(appName); err != nil {
		return
	}
	parsed.applicationName = appName

	// Look at input.CharmBase and input.CharmSeries for an operating
	// system to deploy with. Only one is allowed and Charm Base is
	// preferred. Luckily, the DeduceOrigin method returns an origin which
	// does contain the base and a series.
	var userSuppliedBase corebase.Base
	if input.CharmBase != "" {
		userSuppliedBase, err = corebase.ParseBaseFromString(input.CharmBase)
		if err != nil {
			return
		}
	} else if input.CharmSeries != "" {
		return parsed, jujuerrors.New("series not supported")
	}
	parsed.charmBase = userSuppliedBase

	placements := []*instance.Placement{}
	if input.Placement == "" {
		placements = nil
	} else {
		placementDirectives := strings.Split(input.Placement, ",")
		// force this to be sorted
		sort.Strings(placementDirectives)

		for _, directive := range placementDirectives {
			appPlacement, err := instance.ParsePlacement(directive)
			if err != nil {
				return parsed, err
			}
			placements = append(placements, appPlacement)
		}
	}

	for _, machine := range input.Machines {
		appPlacement, err := instance.ParsePlacement(machine)
		if err != nil {
			return parsed, err
		}
		placements = append(placements, appPlacement)
	}
	parsed.placement = placements

	// remove this validation once the provider bug lp#2055868
	// is fixed.
	endpointBindings := map[string]string{}
	if len(input.EndpointBindings) > 0 {
		spaceAPIClient := apispaces.NewAPI(conn)
		knownSpaces, err := spaceAPIClient.ListSpaces(ctx)
		if err != nil {
			return parsed, err
		}
		knownSpaceNames := set.NewStrings()
		for _, space := range knownSpaces {
			knownSpaceNames.Add(space.Name)
		}
		for endpoint, space := range input.EndpointBindings {
			if !knownSpaceNames.Contains(space) {
				return parsed, fmt.Errorf("unknown space %q", space)
			}
			endpointBindings[endpoint] = space
		}
	}
	parsed.endpointBindings = endpointBindings

	return
}

type transformedCreateApplicationInput struct {
	applicationName  string
	charmName        string
	charmChannel     string
	charmBase        corebase.Base
	charmRevision    int
	config           map[string]string
	constraints      constraints.Value
	expose           map[string]interface{}
	placement        []*instance.Placement
	units            int
	trust            bool
	endpointBindings map[string]string
	resources        map[string]CharmResource
	storage          map[string]jujustorage.Directive
}

type CreateApplicationResponse struct {
	AppName   string
	ModelType string
}

type ReadApplicationInput struct {
	ModelUUID string
	AppName   string
}

type ReadApplicationResponse struct {
	Name             string
	Channel          string
	Revision         int
	Base             string
	ModelType        string
	Series           string
	Units            int
	Trust            bool
	Config           map[string]ConfigEntry
	Constraints      constraints.Value
	Expose           map[string]interface{}
	Principal        bool
	Placement        string
	Machines         []string
	EndpointBindings map[string]string
	Storage          map[string]jujustorage.Directive
	Resources        map[string]string
}

type UpdateApplicationInput struct {
	ModelUUID string
	ModelInfo *params.ModelInfo
	AppName   string
	Units     *int
	Revision  *int
	Channel   string
	Trust     *bool
	Expose    map[string]interface{}
	// Unexpose indicates what endpoints to unexpose
	Unexpose          []string
	Config            map[string]string
	UnsetConfig       []string
	Base              string
	Constraints       *constraints.Value
	EndpointBindings  map[string]string
	StorageDirectives map[string]jujustorage.Directive
	Resources         map[string]CharmResource
	AddMachines       []string
	RemoveMachines    []string
}

type DestroyApplicationInput struct {
	ApplicationName string
	ModelUUID       string
}

func resolveCharmURL(charmName string) (*charm.URL, error) {
	path, err := charm.EnsureSchema(charmName, charm.CharmHub)
	if err != nil {
		return nil, err
	}
	charmURL, err := charm.ParseURL(path)
	if err != nil {
		return nil, err
	}

	return charmURL, nil
}

func (c applicationsClient) CreateApplication(ctx context.Context, input *CreateApplicationInput) (*CreateApplicationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	transformedInput, err := input.validateAndTransform(ctx, conn)
	if err != nil {
		return nil, err
	}

	applicationAPIClient := apiapplication.NewClient(conn)
	resourceAPIClient, err := apiresources.NewClient(conn)
	if err != nil {
		return nil, err
	}
	if applicationAPIClient.BestAPIVersion() >= 19 {
		err := c.deployFromRepository(ctx, applicationAPIClient, resourceAPIClient, transformedInput)
		if err != nil {
			return nil, err
		}
	} else {
		err := c.legacyDeploy(ctx, conn, applicationAPIClient, transformedInput)
		if err != nil {
			return nil, jujuerrors.Annotate(err, "legacy deploy method")
		}
	}

	// If we have managed to deploy something, now we have
	// to check if we have to expose something
	err = c.processExpose(ctx, applicationAPIClient, transformedInput.applicationName, transformedInput.expose)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", newApplicationPartiallyCreatedError(transformedInput.applicationName), err)
	}
	modelType, err := c.ModelType(input.ModelUUID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", newApplicationPartiallyCreatedError(transformedInput.applicationName), err)
	}
	return &CreateApplicationResponse{
		AppName:   transformedInput.applicationName,
		ModelType: modelType.String(),
	}, nil
}

func (c applicationsClient) deployFromRepository(ctx context.Context, applicationAPIClient ApplicationAPIClient, resourceAPIClient ResourceAPIClient, transformedInput transformedCreateApplicationInput) error {
	settingsForYaml := map[interface{}]interface{}{transformedInput.applicationName: transformedInput.config}
	configYaml, err := goyaml.Marshal(settingsForYaml)
	if err != nil {
		return jujuerrors.Trace(err)
	}
	c.Tracef("Calling DeployFromRepository")
	deployInfo, localPendingResources, errs := applicationAPIClient.DeployFromRepository(ctx, apiapplication.DeployFromRepositoryArg{
		CharmName:        transformedInput.charmName,
		ApplicationName:  transformedInput.applicationName,
		Base:             &transformedInput.charmBase,
		Channel:          &transformedInput.charmChannel,
		ConfigYAML:       string(configYaml),
		Cons:             transformedInput.constraints,
		EndpointBindings: transformedInput.endpointBindings,
		NumUnits:         &transformedInput.units,
		Placement:        transformedInput.placement,
		Revision:         &transformedInput.charmRevision,
		Trust:            transformedInput.trust,
		Resources:        resourcesAsStringMap(transformedInput.resources),
		Storage:          transformedInput.storage,
	})

	if len(errs) != 0 {
		return stderrors.Join(errs...)
	}

	// Upload the provided local resources to Juju
	uploadErr := uploadExistingPendingResources(ctx, deployInfo.Name, localPendingResources, transformedInput.resources, resourceAPIClient)

	if uploadErr != nil {
		return fmt.Errorf("%w: %w", newApplicationPartiallyCreatedError(transformedInput.applicationName), uploadErr)
	}
	return nil
}

// resourcesAsStringMap converts our strongly typed resource map into one appropriate
// for use with the Juju API. The string map holds "<resoure-name>":"<value>"
// where the value is either a revision number or an arbitrary string.
//
// In the `DeployFromRepository` API, if the value is not a revision number, Juju will return
// the arbitrary string back to us in the "filename" field of the resourcesToUpload since
// the Juju CLI conventionally fetches this information from a local file.
func resourcesAsStringMap(resources map[string]CharmResource) map[string]string {
	result := map[string]string{}
	for resourceName, resource := range resources {
		result[resourceName] = resource.String()
	}
	return result
}

// TODO (hml) 23-Feb-2024
// Remove the functionality associated with legacyDeploy
// once the provider no longer supports a version of juju
// before 3.3.
func (c applicationsClient) legacyDeploy(ctx context.Context, conn api.Connection, applicationAPIClient *apiapplication.Client, transformedInput transformedCreateApplicationInput) error {
	// Version needed for operating system selection.
	c.controllerVersion, _ = conn.ServerVersion()

	charmsAPIClient := apicharms.NewClient(conn)
	modelconfigAPIClient := apimodelconfig.NewClient(conn)

	channel, err := charm.ParseChannel(transformedInput.charmChannel)
	if err != nil {
		return err
	}

	charmURL, err := resolveCharmURL(transformedInput.charmName)
	if err != nil {
		return err
	}

	subordinate, err := c.getCharmClient(conn).IsSubordinateCharm(ctx, IsSubordinateCharmParameters{
		Name:    transformedInput.charmName,
		Channel: transformedInput.charmChannel,
	})
	if err != nil {
		return err
	}
	if subordinate {
		transformedInput.units = 0
	}

	if charmURL.Revision != UnspecifiedRevision {
		return fmt.Errorf("cannot specify revision in a charm name")
	}
	if transformedInput.charmRevision != UnspecifiedRevision && channel.Empty() {
		return fmt.Errorf("specifying a revision requires a channel for future upgrades")
	}

	userSuppliedBase := transformedInput.charmBase
	platformCons, err := modelconfigAPIClient.GetModelConstraints(ctx)
	if err != nil {
		return err
	}
	platform := utils.MakePlatform(transformedInput.constraints, userSuppliedBase, platformCons)

	urlForOrigin := charmURL
	if transformedInput.charmRevision != UnspecifiedRevision {
		urlForOrigin = urlForOrigin.WithRevision(transformedInput.charmRevision)
	}

	// Juju 2.9 cares that the series is in the origin. Juju 3.3 does not.
	// We are supporting both now.
	if !userSuppliedBase.Empty() {
		return jujuerrors.New("series not supported")
	}

	origin, err := utils.MakeOrigin(charm.Schema(urlForOrigin.Schema), transformedInput.charmRevision, channel, platform)
	if err != nil {
		return err
	}

	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolvedURL, resolvedOrigin, supportedBases, err := resolveCharm(ctx, charmsAPIClient, charmURL, origin)
	if err != nil {
		return err
	}
	if resolvedOrigin.Type == "bundle" {
		return jujuerrors.NotSupportedf("deploying bundles")
	}
	c.Tracef("resolveCharm returned", map[string]interface{}{"resolvedURL": resolvedURL, "resolvedOrigin": resolvedOrigin, "supportedBases": supportedBases})

	baseToUse, err := c.baseToUse(ctx, modelconfigAPIClient, userSuppliedBase, resolvedOrigin.Base, supportedBases)
	if err != nil {
		c.Warnf("failed to get a suggested operating system from resolved charm response", map[string]interface{}{"err": err})
	}
	// Double check we got what was requested.
	if !userSuppliedBase.Empty() && !userSuppliedBase.IsCompatible(baseToUse) {
		return jujuerrors.Errorf(
			"juju bug (LP 2039179), requested base %q does not match base %q found for charm.",
			userSuppliedBase, baseToUse)
	}
	resolvedOrigin.Base = baseToUse

	appConfig := transformedInput.config
	if appConfig == nil {
		appConfig = make(map[string]string)
	}
	appConfig["trust"] = fmt.Sprintf("%v", transformedInput.trust)

	// If a plan element, with RequiresReplace in the schema, is
	// changed. Terraform calls the Destroy method then the Create
	// method for resource. This provider does not wait for Destroy
	// to be complete before returning. Therefore, a race may occur
	// of tearing down and reading the same charm.
	//
	// Do the actual work to create an application within Retry.
	// Errors seen so far include:
	// * cannot add application "replace": charm "ch:amd64/jammy/mysql-196" not found
	// * cannot add application "replace": application already exists
	// * cannot add application "replace": charm: not found or not alive
	return retry.Call(retry.CallArgs{
		Func: func() error {
			c.Tracef("AddCharm ", map[string]interface{}{"resolvedURL": resolvedURL, "resolvedOrigin": resolvedOrigin})
			resultOrigin, err := charmsAPIClient.AddCharm(ctx, resolvedURL, resolvedOrigin, false)
			if err != nil {
				err2 := typedError(err)
				// If the charm is AlreadyExists, keep going, we
				// may still be able to create the application. It's
				// also possible we have multiple applications using
				// the same charm.
				if !jujuerrors.Is(err2, jujuerrors.AlreadyExists) {
					return err2
				}
			}

			charmID := apiapplication.CharmID{
				URL:    resolvedURL.String(),
				Origin: resultOrigin,
			}

			resources, err := c.processResources(ctx, charmsAPIClient, conn, charmID, transformedInput.applicationName, transformedInput.resources)
			if err != nil && !jujuerrors.Is(err, jujuerrors.AlreadyExists) {
				return err
			}

			args := apiapplication.DeployArgs{
				CharmID:          charmID,
				ApplicationName:  transformedInput.applicationName,
				NumUnits:         transformedInput.units,
				CharmOrigin:      resultOrigin,
				Config:           appConfig,
				Cons:             transformedInput.constraints,
				Resources:        resources,
				Storage:          transformedInput.storage,
				Placement:        transformedInput.placement,
				EndpointBindings: transformedInput.endpointBindings,
			}
			c.Tracef("Calling Deploy", map[string]interface{}{"args": args})
			if err = applicationAPIClient.Deploy(ctx, args); err != nil {
				return typedError(err)
			}
			return nil
		},
		IsFatalError: func(err error) bool {
			// If we hit AlreadyExists, it is from Deploy only under 2
			// scenarios:
			//   1. User error, the application has already been created?
			//   2. We're replacing the application and tear down hasn't
			//      finished yet, we should try again.
			return !jujuerrors.Is(err, jujuerrors.NotFound) && !jujuerrors.Is(err, jujuerrors.AlreadyExists)
		},
		NotifyFunc: func(err error, attempt int) {
			c.Errorf(err, fmt.Sprintf("deploy application %q retry", transformedInput.applicationName))
			message := fmt.Sprintf("waiting for application %q deploy, attempt %d", transformedInput.applicationName, attempt)
			c.Debugf(message)
		},
		BackoffFunc: retry.DoubleDelay,
		Attempts:    30,
		Delay:       time.Second,
		Clock:       clock.WallClock,
		Stop:        ctx.Done(),
	})
}

// supportedWorkloadBase returns a slice of supported workload basees
// depending on the controller agent version. This provider currently
// uses juju 3.3.0 code. However, the supported workload base list is
// different between juju 2 and juju 3. Handle that here.
func (c applicationsClient) supportedWorkloadBase(imageStream string) ([]corebase.Base, error) {
	supportedBases := corebase.WorkloadBases()
	if c.controllerVersion.Major > 2 {
		// SupportedBases include those supported with juju 3.x; juju 2.9.x
		// supports more. If we have a juju 2.9.x controller add them back.
		additionallySupported := []corebase.Base{
			{OS: "ubuntu", Channel: corebase.Channel{Track: "18.04"}}, // bionic
			{OS: "ubuntu", Channel: corebase.Channel{Track: "16.04"}}, // xenial
			{OS: "ubuntu", Channel: corebase.Channel{Track: "14.04"}}, // trusty
			{OS: "ubuntu", Channel: corebase.Channel{Track: "12.04"}}, // precise
			{OS: "windows"},
			{OS: "centos", Channel: corebase.Channel{Track: "7"}}, // centos7
		}
		supportedBases = append(supportedBases, additionallySupported...)
	}
	return supportedBases, nil
}

// baseToUse selects a base to deploy a charm with based on the following
// criteria
//   - A user specified base must be supported by the charm and a valid juju
//     supported workload base. If so, use that, otherwise if an input base
//     is provided, return an error.
//   - Next check DefaultBase from model config. If explicitly defined by the
//     user, check against charm and juju supported workloads. Use that if in
//     both lists.
//   - Third check the suggested base.
//   - Fourth, use the DefaultLTS if a supported base.
//   - Lastly, pop the first element of the supported bases off the list and use
//     that.
//
// If the intersection of the charm and supported workload bases is empty, exit
// with an error.
//
// Note, we are re-implementing the logic of base_selector in juju code as it's
// a private object.
func (c applicationsClient) baseToUse(ctx context.Context, modelconfigAPIClient *apimodelconfig.Client, inputBase, suggestedBase corebase.Base, charmBases []corebase.Base) (corebase.Base, error) {
	c.Tracef("baseToUse", map[string]interface{}{"inputBase": inputBase, "suggestedBase": suggestedBase, "charmBases": charmBases})

	attrs, err := modelconfigAPIClient.ModelGet(ctx)
	if err != nil {
		return corebase.Base{}, jujuerrors.Wrap(err, jujuerrors.New("cannot fetch model settings"))
	}
	modelConfig, err := config.New(config.NoDefaults, attrs)
	if err != nil {
		return corebase.Base{}, err
	}

	supportedWorkloadBases, err := c.supportedWorkloadBase(modelConfig.ImageStream())
	if err != nil {
		return corebase.Base{}, err
	}

	// We can choose from a list of bases, supported both as
	// workload bases and by the charm.
	supportedBases := intersectionOfBases(charmBases, supportedWorkloadBases)
	if len(supportedBases) == 0 {
		return corebase.Base{}, jujuerrors.NewNotSupported(nil,
			"This charm has no bases supported by the charm and in the list of juju workload bases for the current version of juju.")
	}

	// If the inputBase is supported by the charm and is a supported
	// workload base, use that.
	if basesContain(inputBase, supportedBases) {
		return inputBase, nil
	} else if !inputBase.Empty() {
		return corebase.Base{}, jujuerrors.NewNotSupported(nil,
			fmt.Sprintf("base %q either not supported by the charm, or an unsupported juju workload base with the current version of juju.", inputBase))
	}

	// If a default base is explicitly defined for the model,
	// use that if a supportedBase.
	defaultBaseString, explicit := modelConfig.DefaultBase()
	if explicit {
		defaultBase, err := corebase.ParseBaseFromString(defaultBaseString)
		if err != nil {
			return corebase.Base{}, err
		}
		if basesContain(defaultBase, supportedBases) {
			return defaultBase, nil
		}
	}

	// If a suggested base is in the supportedBases list, use it.
	if basesContain(suggestedBase, supportedBases) {
		return suggestedBase, nil
	}

	// Note: This DefaultSupportedLTSBase is specific to juju 3.3.0
	lts := jujuversion.DefaultSupportedLTSBase()
	if basesContain(lts, supportedBases) {
		return lts, nil
	}

	// Last attempt, the first base in supported Bases.
	return supportedBases[0], nil
}

// processExpose is a local function that executes an exposed request.
// If the exposeConfig argument is nil it simply exits. If not,
// an exposed request is done populating the request arguments with
// the endpoints, spaces, and cidrs contained in the exposeConfig
// map.
func (c applicationsClient) processExpose(ctx context.Context, applicationAPIClient ApplicationAPIClient, applicationName string, expose map[string]interface{}) error {
	// nothing to do
	if expose == nil {
		return nil
	}

	exposeConfig := make(map[string]string)
	for k, v := range expose {
		if v != nil {
			exposeConfig[k] = v.(string)
		} else {
			exposeConfig[k] = ""
		}
	}

	// create one entry with spaces and the CIDRs per endpoint. If no endpoint
	// use an empty value ("")
	listEndpoints := splitCommaDelimitedList(exposeConfig["endpoints"])
	listSpaces := splitCommaDelimitedList(exposeConfig["spaces"])
	listCIDRs := splitCommaDelimitedList(exposeConfig["cidrs"])

	if len(listEndpoints)+len(listSpaces)+len(listCIDRs) == 0 {
		c.Tracef(fmt.Sprintf("call expose application [%s]", applicationName))
		return applicationAPIClient.Expose(ctx, applicationName, nil)
	}

	// build params and send the request
	if len(listEndpoints) == 0 {
		listEndpoints = append(listEndpoints, "")
	}

	requestParams := make(map[string]params.ExposedEndpoint)
	for _, epName := range listEndpoints {
		requestParams[epName] = params.ExposedEndpoint{
			ExposeToSpaces: listSpaces,
			ExposeToCIDRs:  listCIDRs,
		}
	}

	c.Tracef("call expose API endpoint", map[string]interface{}{"ExposeParams": requestParams})

	return applicationAPIClient.Expose(ctx, applicationName, requestParams)
}

func splitCommaDelimitedList(list string) []string {
	items := make([]string, 0)
	for _, token := range strings.Split(list, ",") {
		token = strings.TrimSpace(token)
		if len(token) == 0 {
			continue
		}
		items = append(items, token)
	}
	return items
}

// processResources is a helper function to process the charm
// metadata and request the download of any additional resource.
func (c applicationsClient) processResources(ctx context.Context, charmsAPIClient *apicharms.Client, conn api.Connection, charmID apiapplication.CharmID, appName string, resourcesToUse map[string]CharmResource) (map[string]string, error) {
	charmInfo, err := charmsAPIClient.CharmInfo(ctx, charmID.URL)
	if err != nil {
		return nil, typedError(err)
	}

	// check if we have resources to request
	if len(charmInfo.Meta.Resources) == 0 && len(resourcesToUse) == 0 {
		return nil, nil
	}

	resourcesAPIClient, err := c.getResourceAPIClient(conn)
	if err != nil {
		return nil, err
	}
	return addPendingResources(ctx, appName, charmInfo.Meta.Resources, resourcesToUse, charmID, resourcesAPIClient)
}

// ReadApplicationWithRetryOnNotFound calls ReadApplication until
// successful, or the count is exceeded when the error is of type
// not found. Delay indicates how long to wait between attempts.
func (c applicationsClient) ReadApplicationWithRetryOnNotFound(ctx context.Context, input *ReadApplicationInput) (*ReadApplicationResponse, error) {
	var output *ReadApplicationResponse
	modelType, err := c.ModelType(input.ModelUUID)
	if err != nil {
		return nil, jujuerrors.Annotatef(err, "getting model type")
	}
	retryErr := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			output, err = c.ReadApplication(ctx, input)
			if jujuerrors.As(err, &ApplicationNotFoundError) || jujuerrors.As(err, &StorageNotFoundError) {
				return err
			} else if err != nil {
				return err
			}

			// NOTE: Applications can always have storage. However, they
			// will not be listed right after the application is created. So
			// we need to wait for the storage to be ready. And we need to
			// check if all storage constraints have pool equal "" and size equal 0
			// to drop the error.
			for label, storage := range output.Storage {
				if storage.Pool == "" || storage.Size == 0 {
					return NewRetryReadError(
						fmt.Sprintf("storage label %q missing detail", label),
					)
				}
			}

			// NOTE: An IAAS subordinate should also have machines. However, they
			// will not be listed until after the relation has been created.
			// Those happen with the integration resource which will not be
			// run by terraform before the application resource finishes. Thus
			// do not block here for subordinates.
			if modelType != model.IAAS || !output.Principal || output.Units == 0 {
				// No need to wait for machines in these cases.
				return nil
			}
			if output.Placement == "" {
				return NewRetryReadError("no machines found in output")
			}
			machines := strings.Split(output.Placement, ",")
			if len(machines) != output.Units {
				return NewRetryReadError(
					fmt.Sprintf("expected %d machines, got %d", output.Units, len(machines)),
				)
			}

			c.Tracef("Have machines - returning", map[string]interface{}{"output": *output})
			return nil
		},
		IsFatalError: func(err error) bool {
			if jujuerrors.Is(err, ApplicationNotFoundError) ||
				jujuerrors.Is(err, StorageNotFoundError) ||
				jujuerrors.Is(err, RetryReadError) ||
				strings.Contains(err.Error(), "connection refused") {
				return false
			}
			return true
		},
		NotifyFunc: func(err error, attempt int) {
			if attempt%4 == 0 {
				message := fmt.Sprintf("waiting for application %q", input.AppName)
				if attempt != 4 {
					message = "still " + message
				}
				c.Debugf(message, map[string]interface{}{"err": err})
			}
		},
		BackoffFunc: retry.DoubleDelay,
		Attempts:    30,
		Delay:       time.Second,
		Clock:       clock.WallClock,
		Stop:        ctx.Done(),
	})
	return output, retryErr
}

func (c *applicationsClient) applicationStorageDirectives(status params.FullStatus, appStatus params.ApplicationStatus) map[string]jujustorage.Directive {
	// first we collect all application units
	appUnits := make(map[string]bool)
	for unitTag := range appStatus.Units {
		appUnits[unitTag] = true
	}

	// then do the filtering
	storageDetailsSlice := []params.StorageDetails{}
	filesystemDetailsSlice := []params.FilesystemDetails{}
	volumeDetailsSlice := []params.VolumeDetails{}
	for _, storageDetails := range status.Storage {
		isAppStorage := false
		for unitTag := range storageDetails.Attachments {
			if appUnits[unitTag] {
				isAppStorage = true
				break
			}
		}
		if isAppStorage {
			storageDetailsSlice = append(storageDetailsSlice, storageDetails)
		}
	}

	for _, filesystemDetails := range status.Filesystems {
		isAppFilesystem := false
		for unitTag := range filesystemDetails.UnitAttachments {
			if appUnits[unitTag] {
				isAppFilesystem = true
				break
			}
		}
		if isAppFilesystem {
			filesystemDetailsSlice = append(filesystemDetailsSlice, filesystemDetails)
		}
	}

	for _, volumeDetails := range status.Volumes {
		isAppVolume := false
		for unitTag := range volumeDetails.UnitAttachments {
			if appUnits[unitTag] {
				isAppVolume = true
				break
			}
		}
		if isAppVolume {
			volumeDetailsSlice = append(volumeDetailsSlice, volumeDetails)
		}
	}

	return c.transformToStorageDirectives(storageDetailsSlice, filesystemDetailsSlice, volumeDetailsSlice)
}

func (c *applicationsClient) transformToStorageDirectives(
	storageDetailsSlice []params.StorageDetails,
	filesystemDetailsSlice []params.FilesystemDetails,
	volumeDetailsSlice []params.VolumeDetails,
) map[string]jujustorage.Directive {
	storageDirectives := make(map[string]jujustorage.Directive)
	for _, storageDetails := range storageDetailsSlice {
		// switch base on storage kind
		storageCounters := make(map[string]uint64)
		switch storageDetails.Kind.String() {
		case "filesystem":
			for _, fd := range filesystemDetailsSlice {
				if fd.Storage == nil {
					c.Debugf("nil storage pointer for filesystems",
						map[string]interface{}{
							storageDetails.StorageTag: storageDetails.Status.Status.String(),
							fd.FilesystemTag:          fd.Status.Status.String(),
						})
					continue
				}
				if fd.Storage.StorageTag == storageDetails.StorageTag {
					// Cut PrefixStorage from the storage tag and `-NUMBER` suffix
					storageLabel := getStorageLabel(storageDetails.StorageTag)
					storageCounters[storageLabel]++
					storageDirectives[storageLabel] = jujustorage.Directive{
						Pool:  fd.Info.Pool,
						Size:  fd.Info.SizeMiB,
						Count: storageCounters[storageLabel],
					}
				}
			}
		case "block":
			for _, vd := range volumeDetailsSlice {
				if vd.Storage == nil {
					c.Debugf("nil storage pointer for volumes",
						map[string]interface{}{
							storageDetails.StorageTag: storageDetails.Status.Status.String(),
							vd.VolumeTag:              vd.Status.Status.String(),
						})
					continue
				}
				if vd.Storage.StorageTag == storageDetails.StorageTag {
					storageLabel := getStorageLabel(storageDetails.StorageTag)
					storageCounters[storageLabel]++
					storageDirectives[storageLabel] = jujustorage.Directive{
						Pool:  vd.Info.Pool,
						Size:  vd.Info.SizeMiB,
						Count: storageCounters[storageLabel],
					}
				}
			}
		}
	}
	return storageDirectives
}

func getStorageLabel(storageTag string) string {
	return strings.TrimSuffix(strings.TrimPrefix(storageTag, PrefixStorage), "-0")
}

func (c applicationsClient) ReadApplication(ctx context.Context, input *ReadApplicationInput) (*ReadApplicationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	applicationAPIClient := c.getApplicationAPIClient(conn)
	modelconfigAPIClient := c.getModelConfigAPIClient(conn)

	apps, err := applicationAPIClient.ApplicationsInfo(ctx, []names.ApplicationTag{names.NewApplicationTag(input.AppName)})
	if err != nil {
		return nil, jujuerrors.Annotate(err, "when querying the applications info")
	}
	if len(apps) > 1 {
		return nil, fmt.Errorf("more than one result for application: %s", input.AppName)
	}
	if len(apps) < 1 {
		return nil, NewApplicationNotFoundError(input.AppName)
	}
	if apps[0].Error != nil {
		// Return applicationNotFoundError to trigger retry.
		c.Debugf("Actual error from ApplicationsInfo", map[string]interface{}{"err": apps[0].Error})
		return nil, NewApplicationNotFoundError(input.AppName)
	}

	appInfo := apps[0].Result

	var appStatus params.ApplicationStatus
	var storageDirectives map[string]jujustorage.Directive
	if c.controllerVersion.Major == 4 {
		// With Juju 4 we have to manually filter storage/volumes/filesystems of an application.
		appStatus, storageDirectives, err = c.getApplicationStatusAndStorageDirectives4(ctx, conn, input.AppName)
		if err != nil {
			return nil, err
		}
	} else {
		appStatus, storageDirectives, err = c.getApplicationStatusAndStorageDirectives(ctx, conn, input.AppName)
		if err != nil {
			return nil, err
		}
	}

	allocatedMachines := set.NewStrings()
	for _, v := range appStatus.Units {
		if v.Machine != "" {
			allocatedMachines.Add(v.Machine)
		}
	}
	machines := allocatedMachines.SortedValues()

	var placement string
	if !allocatedMachines.IsEmpty() {
		placement = strings.Join(allocatedMachines.SortedValues(), ",")
	}

	unitCount := len(appStatus.Units)
	// if we have a CAAS we use scale instead of units length
	modelType, err := c.ModelType(input.ModelUUID)
	if err != nil {
		return nil, err
	}
	if modelType == model.CAAS {
		unitCount = appStatus.Scale
	}

	// NOTE: we are assuming that this charm comes from CharmHub
	charmURL, err := charm.ParseURL(appStatus.Charm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse charm: %v", err)
	}

	returnedConf, err := applicationAPIClient.Get(ctx, input.AppName)
	if err != nil {
		return nil, fmt.Errorf("failed to get app configuration %v", err)
	}

	conf := make(map[string]ConfigEntry, 0)
	if returnedConf.ApplicationConfig != nil {
		for k, v := range returnedConf.ApplicationConfig {
			// skip the trust value. We have an independent field for that
			if k == "trust" {
				continue
			}
			// The API returns the configuration entries as interfaces
			aux := v.(map[string]interface{})
			// set if we find the value key and this is not a default
			// value.
			if value, found := aux["value"]; found {
				conf[k] = ConfigEntry{
					Value:     value,
					IsDefault: aux["source"] == "default",
				}
			}
		}
		// repeat the same steps for charm config values
		for k, v := range returnedConf.CharmConfig {
			aux := v.(map[string]interface{})
			if value, found := aux["value"]; found {
				conf[k] = ConfigEntry{
					Value:     value,
					IsDefault: aux["source"] == "default",
				}
			}
		}
	}

	// trust field which has to be included into the configuration
	trustValue := false
	if returnedConf.ApplicationConfig != nil {
		aux, found := returnedConf.ApplicationConfig["trust"]
		if found {
			m := aux.(map[string]any)
			target, found := m["value"]
			if found {
				trustValue = target.(bool)
			}
		}
	}

	// the expose field requires additional logic because
	// the API returns populated cidrs by default. Additionally,
	// we populate the unexpose field in the response structure
	// to indicate endpoints that has to be removed by comparing
	var exposed map[string]interface{} = nil
	if appStatus.Exposed {
		// rebuild
		exposed = make(map[string]interface{}, 0)
		endpoints := []string{}
		spaces := ""
		cidrs := ""
		for epName, value := range appStatus.ExposedEndpoints {
			if epName != "" {
				endpoints = append(endpoints, epName)
			}
			if len(spaces) == 0 {
				spaces = strings.Join(value.ExposeToSpaces, ",")
			}
			if len(cidrs) == 0 {
				// by default the API sets
				// cidrs: "0.0.0.0/0,::/0"
				// ignore them
				aux := removeDefaultCidrs(value.ExposeToCIDRs)
				cidrs = strings.Join(aux, ",")
			}
		}
		if len(endpoints) > 0 {
			slices.Sort(endpoints)
			exposed["endpoints"] = strings.Join(endpoints, ",")
		} else {
			exposed["endpoints"] = ""
		}
		exposed["spaces"] = spaces
		exposed["cidrs"] = cidrs
	}
	// ParseChannel to send back a base without the risk.
	// Having the risk will cause issues with the provider
	// saving a different value than the user did.
	baseChannel, err := corebase.ParseChannel(appInfo.Base.Channel)
	if err != nil {
		return nil, jujuerrors.Annotate(err, "failed parse channel for base")
	}

	defaultSpace, err := getModelDefaultSpace(ctx, modelconfigAPIClient)
	if err != nil {
		return nil, err
	}
	appDefaultSpace := appStatus.EndpointBindings[""]
	if appDefaultSpace == "" {
		appDefaultSpace = defaultSpace
	}

	endpointBindings := make(map[string]string)
	if appDefaultSpace != defaultSpace {
		endpointBindings[""] = appDefaultSpace
	}
	for endpoint, space := range appStatus.EndpointBindings {
		if endpoint != "" && space != appDefaultSpace {
			endpointBindings[endpoint] = space
		}
	}

	resourcesAPIClient, err := c.getResourceAPIClient(conn)
	if err != nil {
		return nil, err
	}
	resources, err := resourcesAPIClient.ListResources(ctx, []string{input.AppName})
	if err != nil {
		return nil, jujuerrors.Annotate(err, "failed to list application resources")
	}
	usedResources := make(map[string]string)
	for _, iResources := range resources {
		for _, resource := range iResources.Resources {
			// Per juju convention, -1, indicates that an integer value has not been set.
			// Uploaded resources currently have no revision number.
			// So when the revision number is -1, we can use the value in state.
			if resource.Origin == charmresources.OriginUpload {
				usedResources[resource.Name] = "-1"
			} else {
				usedResources[resource.Name] = strconv.Itoa(resource.Revision)
			}
		}
	}

	response := &ReadApplicationResponse{
		Name:             charmURL.Name,
		Channel:          appInfo.Channel,
		Revision:         charmURL.Revision,
		Base:             fmt.Sprintf("%s@%s", appInfo.Base.Name, baseChannel.Track),
		ModelType:        modelType.String(),
		Units:            unitCount,
		Trust:            trustValue,
		Expose:           exposed,
		Config:           conf,
		Constraints:      appInfo.Constraints,
		Principal:        appInfo.Principal,
		Placement:        placement,
		Machines:         machines,
		EndpointBindings: endpointBindings,
		Storage:          storageDirectives,
		Resources:        usedResources,
	}

	return response, nil
}

func (c applicationsClient) getApplicationStatusAndStorageDirectives4(ctx context.Context, conn api.Connection, appName string) (params.ApplicationStatus, map[string]jujustorage.Directive, error) {
	clientAPIClient := c.getClientAPIClient(conn)

	// Fetch status. Storage is not provided by application,
	// rather storage data buries a unit name deep
	// in the structure.
	// TODO(alesstimec): Switch to using GetApplicationStorage once juju 4 implements it.
	status, err := clientAPIClient.Status(ctx, &apiclient.StatusArgs{
		IncludeStorage: true,
	})
	if err != nil {
		c.Errorf(err, "failed to get status")
		return params.ApplicationStatus{}, nil, err
	}

	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[appName]; !exists {
		return params.ApplicationStatus{}, nil, fmt.Errorf("no status returned for application: %s", appName)
	}

	return appStatus, c.applicationStorageDirectives(*status, appStatus), nil
}

func (c applicationsClient) getApplicationStatusAndStorageDirectives(ctx context.Context, conn api.Connection, appName string) (params.ApplicationStatus, map[string]jujustorage.Directive, error) {
	clientAPIClient := c.getClientAPIClient(conn)

	// Fetch data only about the application being read. This helps to limit
	// the data on storage to the specific application too. Storage is not
	// provided by application, rather storage data buries a unit name deep
	// in the structure.
	status, err := clientAPIClient.Status(ctx, &apiclient.StatusArgs{
		Patterns:       []string{appName},
		IncludeStorage: true,
	})
	if err != nil {
		if strings.Contains(err.Error(), "filesystem for storage instance") ||
			strings.Contains(err.Error(), "volume for storage instance") ||
			strings.Contains(err.Error(), "cannot convert storage details") {
			// Retry if we get this error. It means the storage is not ready yet.
			return params.ApplicationStatus{}, nil, NewStorageNotFoundError(appName)
		}
		c.Errorf(err, "failed to get status")
		return params.ApplicationStatus{}, nil, err
	}
	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[appName]; !exists {
		return params.ApplicationStatus{}, nil, fmt.Errorf("no status returned for application: %s", appName)
	}

	return appStatus, c.transformToStorageDirectives(status.Storage, status.Filesystems, status.Volumes), nil
}

// removeDefaultCidrs is an auxiliar function to remove
// the "0.0.0.0/0 and ::/0" strings from an array of
// cidrs
func removeDefaultCidrs(cidrs []string) []string {
	toReturn := make([]string, 0)
	for _, cidr := range cidrs {
		if cidr != "0.0.0.0/0" && cidr != "::/0" {
			toReturn = append(toReturn, cidr)
		}
	}
	return toReturn
}

func (c applicationsClient) UpdateApplication(ctx context.Context, input *UpdateApplicationInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	applicationAPIClient := c.getApplicationAPIClient(conn)
	charmsAPIClient := apicharms.NewClient(conn)
	clientAPIClient := c.getClientAPIClient(conn)
	modelconfigAPIClient := c.getModelConfigAPIClient(conn)

	resourcesAPIClient, err := c.getResourceAPIClient(conn)
	if err != nil {
		return err
	}

	status, err := clientAPIClient.Status(ctx, nil)
	if err != nil {
		return err
	}
	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[input.AppName]; !exists {
		return fmt.Errorf("no status returned for application: %s", input.AppName)
	}

	// process configuration
	var auxConfig map[string]string
	if input.Config != nil {
		auxConfig = make(map[string]string)
		for k, v := range input.Config {
			auxConfig[k] = ConfigEntryToString(v)
		}
	}

	// trust goes inside the config
	if input.Trust != nil {
		if auxConfig == nil {
			auxConfig = make(map[string]string)
		}
		auxConfig["trust"] = fmt.Sprintf("%v", *input.Trust)
	}

	err = c.UpdateCharmAndResources(ctx, input, applicationAPIClient, charmsAPIClient, resourcesAPIClient)
	if err != nil {
		c.Errorf(err, "updating charm and resources")
		return err
	}

	if auxConfig != nil {
		err := applicationAPIClient.SetConfig(ctx, input.AppName, "", auxConfig)
		if err != nil {
			c.Errorf(err, "setting configuration params")
			return err
		}
	}

	if len(input.UnsetConfig) > 0 {
		// unset config keys one by one, so we can swallow the `unknown option` error,
		// which means the key was set in the state but is no longer valid (e.g. removed in a new charm revision).
		// We don't want to fail the whole update.
		for _, key := range input.UnsetConfig {
			if err := applicationAPIClient.UnsetApplicationConfig(ctx, input.AppName, []string{key}); err != nil {
				if strings.Contains(err.Error(), "unknown option") {
					continue
				}
				return err
			}
		}
	}

	if len(input.EndpointBindings) > 0 {
		modelDefaultSpace, err := getModelDefaultSpace(ctx, modelconfigAPIClient)
		if err != nil {
			return err
		}
		endpointBindingsParams, err := computeUpdatedBindings(modelDefaultSpace, appStatus.EndpointBindings, input.EndpointBindings, input.AppName)
		if err != nil {
			return err
		}
		err = applicationAPIClient.MergeBindings(ctx, endpointBindingsParams)
		if err != nil {
			c.Errorf(err, "setting endpoint bindings")
			return err
		}
	}

	// unexpose corresponding endpoints
	if len(input.Unexpose) != 0 {
		c.Tracef("Unexposing endpoints", map[string]interface{}{"endpoints": input.Unexpose})
		if err := applicationAPIClient.Unexpose(ctx, input.AppName, input.Unexpose); err != nil {
			c.Errorf(err, "when trying to unexpose")
			return err
		}
	}
	// expose endpoints if required
	if input.Expose != nil {
		c.Tracef("Expose endpoints", map[string]interface{}{"endpoints": input.Unexpose})
		err := c.processExpose(ctx, applicationAPIClient, input.AppName, input.Expose)
		if err != nil {
			c.Errorf(err, "when trying to expose")
			return err
		}
	}

	if input.Constraints != nil {
		err := applicationAPIClient.SetConstraints(ctx, input.AppName, *input.Constraints)
		if err != nil {
			c.Errorf(err, "setting application constraints")
			return err
		}
	}

	// TODO: Refactor this to a separate function
	modelType, err := c.ModelType(input.ModelUUID)
	if err != nil {
		return err
	}
	if input.Units != nil {
		if modelType == model.CAAS {
			_, err := applicationAPIClient.ScaleApplication(ctx, apiapplication.ScaleApplicationParams{
				ApplicationName: input.AppName,
				Scale:           *input.Units,
				Force:           false,
			})
			if err != nil {
				return err
			}
		} else {
			unitDiff := *input.Units - len(appStatus.Units)

			if unitDiff > 0 {
				_, err := applicationAPIClient.AddUnits(
					ctx,
					apiapplication.AddUnitsParams{
						ApplicationName: input.AppName,
						NumUnits:        unitDiff,
					})
				if err != nil {
					return err
				}
			}

			if unitDiff < 0 {
				var unitNames []string
				for unitName := range appStatus.Units {
					unitNames = append(unitNames, unitName)
				}

				unitAbs := int(math.Abs(float64(unitDiff)))
				var unitsToDestroy []string
				for i := 0; i < unitAbs; i++ {
					unitsToDestroy = append(unitsToDestroy, unitNames[i])
				}
				_, err := applicationAPIClient.DestroyUnits(
					ctx,
					apiapplication.DestroyUnitsParams{
						Units:          unitsToDestroy,
						DestroyStorage: true,
					})
				if err != nil {
					return err
				}
			}
		}
	}

	// for IAAS model we process additions/removals of units
	if modelType == model.IAAS {
		if err = c.addUnits(ctx, input, applicationAPIClient); err != nil {
			return err
		}
		if err = c.removeUnits(ctx, input, applicationAPIClient, appStatus); err != nil {
			return err
		}
	}

	return nil
}

func (c applicationsClient) addUnits(ctx context.Context, input *UpdateApplicationInput, client ApplicationAPIClient) error {
	if len(input.AddMachines) != 0 {
		placements := make([]*instance.Placement, len(input.AddMachines))
		for i, machine := range input.AddMachines {
			placement, err := instance.ParsePlacement(machine)
			if err != nil {
				return err
			}
			placements[i] = placement
		}

		_, err := client.AddUnits(ctx, apiapplication.AddUnitsParams{
			ApplicationName: input.AppName,
			NumUnits:        len(input.AddMachines),
			Placement:       placements,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c applicationsClient) removeUnits(ctx context.Context, input *UpdateApplicationInput, client ApplicationAPIClient, appStatus params.ApplicationStatus) error {
	if len(input.RemoveMachines) != 0 {
		machineUnits := make(map[string]string)
		for unitName, unitStatus := range appStatus.Units {
			machineUnits[unitStatus.Machine] = unitName
		}

		unitsToDestroy := []string{}
		for _, machine := range input.RemoveMachines {
			unitName, ok := machineUnits[machine]
			if !ok {
				return fmt.Errorf("no machines deployed on machine: %v", machine)
			}
			unitsToDestroy = append(unitsToDestroy, unitName)
		}
		_, err := client.DestroyUnits(ctx, apiapplication.DestroyUnitsParams{
			Units:          unitsToDestroy,
			DestroyStorage: true,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// UpdateCharmAndResources is a helper function to update the charm and resources
// of an application. It will update the charm or fetch the current one, and
// update the resources if required.
func (c applicationsClient) UpdateCharmAndResources(
	ctx context.Context,
	input *UpdateApplicationInput,
	applicationAPIClient ApplicationAPIClient,
	charmsAPIClient *apicharms.Client,
	resourcesAPIClient ResourceAPIClient,
) error {
	// If the input has no revision, channel, or base, and has resources, we can skip
	// the charm and resources update.
	if input.Revision == nil && input.Channel == "" && input.Base == "" && len(input.Resources) == 0 {
		return nil
	}
	var err error
	var updateCharm bool
	var charmID apiapplication.CharmID
	// Use the revision and channel info to create the
	// corresponding SetCharm info.
	//
	// Note: the operations with revisions should be done
	// before the operations with config. Because the config params
	// can be changed from one revision to another. So "Revision-Config"
	// ordering will help to prevent issues with the configuration parsing.
	if input.Revision != nil || input.Channel != "" || input.Base != "" {
		charmID, err = c.computeCharmID(ctx, input, applicationAPIClient, charmsAPIClient)
		if err != nil {
			return err
		}
		updateCharm = true
	} else {
		// Fetch the current charm URL and origin if the charm is not being updated.
		// This is needed to avoid inadvertently updating the charm when only the
		// resources are being updated.
		url, origin, err := applicationAPIClient.GetCharmURLOrigin(ctx, input.AppName)
		if err != nil {
			return err
		}
		charmID.URL = url.String()
		charmID.Origin = origin
	}

	// Fetch latest resources and update them if the charm is being refreshed
	// or if there are resources to update.
	// Pinned resources will be kept as is.
	var resourceIds map[string]string
	if updateCharm || len(input.Resources) > 0 {
		resourceIds, err = c.updateResources(ctx, input.AppName, input.Resources, charmsAPIClient, charmID, resourcesAPIClient)
		if err != nil {
			return err
		}
		updateCharm = true
	}

	if updateCharm {
		charmConfig := apiapplication.SetCharmConfig{
			ApplicationName:   input.AppName,
			CharmID:           charmID,
			ResourceIDs:       resourceIds,
			StorageDirectives: input.StorageDirectives,
		}
		err = applicationAPIClient.SetCharm(ctx, charmConfig)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c applicationsClient) DestroyApplication(ctx context.Context, input *DestroyApplicationInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	applicationAPIClient := apiapplication.NewClient(conn)

	var destroyParams = apiapplication.DestroyApplicationsParams{
		Applications: []string{
			input.ApplicationName,
		},
		DestroyStorage: true,
	}

	_, err = applicationAPIClient.DestroyApplications(ctx, destroyParams)

	if err != nil {
		return err
	}

	return nil
}

// computeCharmID populates the corresponding CharmID struct
// to indicate juju what charm to be deployed.
func (c applicationsClient) computeCharmID(
	ctx context.Context,
	input *UpdateApplicationInput,
	applicationAPIClient ApplicationAPIClient,
	charmsAPIClient *apicharms.Client,
) (apiapplication.CharmID, error) {
	oldURL, oldOrigin, err := applicationAPIClient.GetCharmURLOrigin(ctx, input.AppName)
	if err != nil {
		return apiapplication.CharmID{}, err
	}
	// You can only refresh on the revision OR the channel at once.
	newURL := oldURL
	newOrigin := oldOrigin
	if input.Revision != nil {
		newURL = oldURL.WithRevision(*input.Revision)
		newOrigin.Revision = input.Revision
		// If the charm has an ID and Hash, it's been deployed before.
		// Remove to trick juju into finding the new revision the user
		// has requested. If they exist, the charm will be resolved with
		// the channel potentially causing the wrong charm revision to
		// be installed.
		//
		// There is a risk if the charm has been renamed in charmhub that
		// the resolve charm will fail as we're using the name instead of
		// the ID. This needs to be fixed in Juju.
		newOrigin.ID = ""
		newOrigin.Hash = ""
	}
	if input.Channel != "" {
		parsedChannel, err := charm.ParseChannel(input.Channel)
		if err != nil {
			return apiapplication.CharmID{}, err
		}
		if parsedChannel.Track != "" {
			newOrigin.Track = strPtr(parsedChannel.Track)
		}
		newOrigin.Risk = string(parsedChannel.Risk)
		if parsedChannel.Branch != "" {
			newOrigin.Branch = strPtr(parsedChannel.Branch)
		}
	}
	if input.Base != "" {
		base, err := corebase.ParseBaseFromString(input.Base)
		if err != nil {
			return apiapplication.CharmID{}, err
		}
		newOrigin.Base = base
	}
	resolvedURL, resolvedOrigin, supportedBases, err := resolveCharm(ctx, charmsAPIClient, newURL, newOrigin)
	if err != nil {
		return apiapplication.CharmID{}, err
	}

	// Ensure that the new charm supports the architecture used by the deployed application.
	if oldOrigin.Architecture != resolvedOrigin.Architecture {
		msg := fmt.Sprintf("the new charm does not support the current architecture %q", oldOrigin.Architecture)
		return apiapplication.CharmID{}, jujuerrors.New(msg)
	}

	// Ensure the new revision or channel is contained
	// in the origin to be saved by juju when AddCharm
	// is called.
	if input.Revision != nil {
		oldOrigin.Revision = input.Revision
		// This code is coupled with the previous `if input.Revision != nil`.
		// The idea is that deploying with ID and Hash set to "" will force Juju to find the revision set by the user.
		oldOrigin.ID = newOrigin.ID
		oldOrigin.Hash = newOrigin.Hash
	}
	if input.Channel != "" {
		oldOrigin.Track = newOrigin.Track
		oldOrigin.Risk = newOrigin.Risk
		oldOrigin.Branch = newOrigin.Branch
	}
	if input.Base != "" {
		oldOrigin.Base = newOrigin.Base
	}

	if !basesContain(oldOrigin.Base, supportedBases) {
		msg := fmt.Sprintf("the new charm does not support the current operating system %q", oldOrigin.Base.String())
		return apiapplication.CharmID{}, jujuerrors.New(msg)
	}

	resultOrigin, err := charmsAPIClient.AddCharm(ctx, resolvedURL, oldOrigin, false)
	if err != nil {
		return apiapplication.CharmID{}, err
	}

	return apiapplication.CharmID{
		URL:    resolvedURL.String(),
		Origin: resultOrigin,
	}, nil
}

func resolveCharm(ctx context.Context, charmsAPIClient *apicharms.Client, curl *charm.URL, origin apicommoncharm.Origin) (*charm.URL, apicommoncharm.Origin, []corebase.Base, error) {
	// Charm or bundle has been supplied as a URL, so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolved, err := charmsAPIClient.ResolveCharms(ctx, []apicharms.CharmToResolve{{URL: curl, Origin: origin}})
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}
	if len(resolved) != 1 {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, fmt.Errorf("expected only one resolution, received %d", len(resolved))
	}
	resolvedCharm := resolved[0]
	return resolvedCharm.URL, resolvedCharm.Origin, resolvedCharm.SupportedBases, resolvedCharm.Error
}

func strPtr(in string) *string {
	return &in
}

func (c applicationsClient) updateResources(ctx context.Context, appName string, resources map[string]CharmResource, charmsAPIClient *apicharms.Client,
	charmID apiapplication.CharmID, resourcesAPIClient ResourceAPIClient) (map[string]string, error) {
	meta, err := utils.GetMetaResources(ctx, charmID.URL, charmsAPIClient)
	if err != nil {
		return nil, err
	}
	filtered, err := utils.GetUpgradeResources(
		ctx,
		charmID,
		charmsAPIClient,
		resourcesAPIClient,
		appName,
		resourcesAsStringMap(resources),
		meta,
	)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	return addPendingResources(ctx, appName, filtered, resources, charmID, resourcesAPIClient)
}

func addPendingResources(ctx context.Context, appName string, charmResourcesToAdd map[string]charmresources.Meta, resourcesToUse map[string]CharmResource,
	charmID apiapplication.CharmID, resourceAPIClient ResourceAPIClient) (map[string]string, error) {
	pendingResourcesforAdd := []charmresources.Resource{}
	resourceIDs := map[string]string{}

	for _, resourceMeta := range charmResourcesToAdd {
		resource, ok := resourcesToUse[resourceMeta.Name]
		if !ok {
			// If there are no resource revisions, the Charm is deployed with
			// default resources according to channel.
			resourceFromCharmhub := charmresources.Resource{
				Meta:     resourceMeta,
				Origin:   charmresources.OriginStore,
				Revision: -1,
			}
			pendingResourcesforAdd = append(pendingResourcesforAdd, resourceFromCharmhub)
			continue
		}

		if providedRev, err := strconv.Atoi(resource.String()); err == nil {
			// A resource revision is provided
			resourceFromCharmhub := charmresources.Resource{
				Meta:   resourceMeta,
				Origin: charmresources.OriginStore,
				// If the resource is removed, providedRev is -1. Then, Charm
				// is deployed with default resources according to channel.
				// Otherwise, Charm is deployed with the provided revision.
				Revision: providedRev,
			}
			pendingResourcesforAdd = append(pendingResourcesforAdd, resourceFromCharmhub)
			continue
		}

		// A new resource to be uploaded by the ResourceApi client.
		localResource := charmresources.Resource{
			Meta:   resourceMeta,
			Origin: charmresources.OriginUpload,
		}
		t, typeParseErr := charmresources.ParseType(resourceMeta.Type.String())
		if typeParseErr != nil {
			return nil, typedError(typeParseErr)
		}
		if t != charmresources.TypeContainerImage { // Uploading a container image implies uploading image metadata.
			// We don't support uploading non-container resources.
			return nil, fmt.Errorf("only container resources can be uploaded; resource %q is of type %q", resourceMeta.Name, t.String())
		}
		details, err := resource.MarhsalYaml()
		if err != nil {
			return nil, typedError(err)
		}
		toRequestUpload, err := resourceAPIClient.UploadPendingResource(
			ctx, apiresources.UploadPendingResourceArgs{
				ApplicationID: appName,
				Resource:      localResource,
				Filename:      resource.String(),
				Reader:        bytes.NewReader(details),
			})
		if err != nil {
			return nil, typedError(err)
		}
		// Add the resource name and the corresponding UUID to the resources map.
		resourceIDs[resourceMeta.Name] = toRequestUpload
	}

	if len(pendingResourcesforAdd) == 0 {
		return resourceIDs, nil
	}

	// Sort the resources by name to ensure consistent ordering.
	// Two resources cannot have the same name.
	slices.SortFunc(pendingResourcesforAdd, func(a, b charmresources.Resource) int {
		return strings.Compare(a.Name, b.Name)
	})

	resourcesReqforAdd := apiresources.AddPendingResourcesArgs{
		ApplicationID: appName,
		CharmID: apiresources.CharmID{
			URL:    charmID.URL,
			Origin: charmID.Origin,
		},
		Resources: pendingResourcesforAdd,
	}
	toRequestAdd, err := resourceAPIClient.AddPendingResources(ctx, resourcesReqforAdd)
	if err != nil {
		return nil, typedError(err)
	}
	// Add the resource name and the corresponding UUID to the resources map
	for i, argsResource := range pendingResourcesforAdd {
		resourceIDs[argsResource.Name] = toRequestAdd[i]
	}

	return resourceIDs, nil
}

func computeUpdatedBindings(modelDefaultSpace string, currentBindings map[string]string, inputBindings map[string]string, appName string) (params.ApplicationMergeBindingsArgs, error) {
	var defaultSpace string
	oldDefault := currentBindings[""]
	newDefaultSpace := inputBindings[""]

	for k := range inputBindings {
		if _, ok := currentBindings[k]; !ok {
			return params.ApplicationMergeBindingsArgs{}, fmt.Errorf("endpoint %q does not exist", k)
		}
	}

	if newDefaultSpace != "" {
		defaultSpace = newDefaultSpace
	} else {
		defaultSpace = modelDefaultSpace
	}

	endpointBindings := make(map[string]string)
	for k, currentSpace := range currentBindings {
		if newSpace, ok := inputBindings[k]; ok {
			if newSpace == "" {
				newSpace = defaultSpace
			}
			endpointBindings[k] = newSpace
		} else {
			if currentSpace == oldDefault {
				endpointBindings[k] = defaultSpace
			} else {
				endpointBindings[k] = currentSpace
			}
		}
	}
	endpointBindingsParams := params.ApplicationMergeBindingsArgs{
		Args: []params.ApplicationMergeBindings{
			{
				ApplicationTag: names.NewApplicationTag(appName).String(),
				Bindings:       endpointBindings,
			},
		},
	}
	return endpointBindingsParams, nil
}

func getModelDefaultSpace(ctx context.Context, modelconfigAPIClient ModelConfigAPIClient) (string, error) {
	attrs, err := modelconfigAPIClient.ModelGet(ctx)
	if err != nil {
		return "", jujuerrors.Annotate(err, "failed to get model config")
	}
	modelConfig, err := config.New(config.UseDefaults, attrs)
	if err != nil {
		return "", jujuerrors.Annotate(err, "failed to cast model config")
	}

	defaultSpace := modelConfig.DefaultSpace()
	if defaultSpace == "" {
		defaultSpace = string(network.AlphaSpaceName)
	}
	return defaultSpace, nil
}
