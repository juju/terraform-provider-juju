// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Package juju is a facade to make interacting with Juju clients simpler. It also acts as an insulating layer
// protecting the provider package from upstream changes.
// The long-term intention is for this package to be removed. Eventually, it would be nice for this package to
// be replaced with more granular clients in Juju itself. Note that much of this code is duplicated from Juju CLI
// commands.
package juju

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/juju/charm/v11"
	charmresources "github.com/juju/charm/v11/resource"
	"github.com/juju/clock"
	"github.com/juju/collections/set"
	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apiclient "github.com/juju/juju/api/client/client"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	apiresources "github.com/juju/juju/api/client/resources"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/cmd/juju/application/utils"
	"github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/instance"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	jujuversion "github.com/juju/juju/version"
	"github.com/juju/names/v4"
	"github.com/juju/retry"
	"github.com/juju/version/v2"
)

var ApplicationNotFoundError = &applicationNotFoundError{}

// ApplicationNotFoundError
type applicationNotFoundError struct {
	appName string
}

func (ae *applicationNotFoundError) Error() string {
	return fmt.Sprintf("application %s not found", ae.appName)
}

type applicationsClient struct {
	SharedClient
	controllerVersion version.Number
}

// ConfigEntry is an auxiliar struct to keep information about
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
	ApplicationName string
	ModelName       string
	CharmName       string
	CharmChannel    string
	CharmBase       string
	CharmSeries     string
	CharmRevision   int
	Units           int
	Trust           bool
	Expose          map[string]interface{}
	Config          map[string]string
	Placement       string
	Constraints     constraints.Value
}

type CreateApplicationResponse struct {
	AppName string
}

type ReadApplicationInput struct {
	ModelName string
	AppName   string
}

type ReadApplicationResponse struct {
	Name        string
	Channel     string
	Revision    int
	Base        string
	Series      string
	Units       int
	Trust       bool
	Config      map[string]ConfigEntry
	Constraints constraints.Value
	Expose      map[string]interface{}
	Principal   bool
	Placement   string
}

type UpdateApplicationInput struct {
	ModelName string
	ModelInfo *params.ModelInfo
	AppName   string
	Units     *int
	Revision  *int
	Channel   string
	Trust     *bool
	Expose    map[string]interface{}
	// Unexpose indicates what endpoints to unexpose
	Unexpose []string
	Config   map[string]string
	//Series    string // Unsupported today
	Placement   map[string]interface{}
	Constraints *constraints.Value
}

type DestroyApplicationInput struct {
	ApplicationName string
	ModelName       string
}

func newApplicationClient(sc SharedClient) *applicationsClient {
	return &applicationsClient{
		SharedClient: sc,
	}
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
	appName := input.ApplicationName
	if appName == "" {
		appName = input.CharmName
	}
	if err := names.ValidateApplicationName(appName); err != nil {
		return nil, err
	}

	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	// Version needed for operating system selection.
	c.controllerVersion, _ = conn.ServerVersion()

	charmsAPIClient := apicharms.NewClient(conn)
	applicationAPIClient := apiapplication.NewClient(conn)
	modelconfigAPIClient := apimodelconfig.NewClient(conn)

	channel, err := charm.ParseChannel(input.CharmChannel)
	if err != nil {
		return nil, err
	}

	charmURL, err := resolveCharmURL(input.CharmName)
	if err != nil {
		return nil, err
	}

	if charmURL.Revision != UnspecifiedRevision {
		return nil, fmt.Errorf("cannot specify revision in a charm name")
	}
	if input.CharmRevision != UnspecifiedRevision && channel.Empty() {
		return nil, fmt.Errorf("specifying a revision requires a channel for future upgrades")
	}

	// Look at input.CharmBase and input.CharmSeries for an operating
	// system to deploy with. Only one is allowed and Charm Base is
	// preferred. Luckily, the DeduceOrigin method returns an origin which
	// does contain the base and a series.
	var userSuppliedBase base.Base
	if input.CharmBase != "" {
		userSuppliedBase, err = base.ParseBaseFromString(input.CharmBase)
		if err != nil {
			return nil, err
		}
	} else if input.CharmSeries != "" {
		userSuppliedBase, err = base.GetBaseFromSeries(input.CharmSeries)
		if err != nil {
			return nil, err
		}
	}
	platformCons, err := modelconfigAPIClient.GetModelConstraints()
	if err != nil {
		return nil, err
	}
	platform := utils.MakePlatform(input.Constraints, userSuppliedBase, platformCons)

	urlForOrigin := charmURL
	if input.CharmRevision != UnspecifiedRevision {
		urlForOrigin = urlForOrigin.WithRevision(input.CharmRevision)
	}

	// Juju 2.9 cares that the series is in the origin. Juju 3.3 does not.
	// We are supporting both now.
	if !userSuppliedBase.Empty() {
		userSuppliedSeries, err := base.GetSeriesFromBase(userSuppliedBase)
		if err != nil {
			return nil, err
		}
		urlForOrigin = urlForOrigin.WithSeries(userSuppliedSeries)
	}

	origin, err := utils.DeduceOrigin(urlForOrigin, channel, platform)
	if err != nil {
		return nil, err
	}

	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolvedURL, resolvedOrigin, supportedBases, err := resolveCharm(charmsAPIClient, charmURL, origin)
	if err != nil {
		return nil, err
	}
	if resolvedOrigin.Type == "bundle" {
		return nil, jujuerrors.NotSupportedf("deploying bundles")
	}

	baseToUse, err := c.baseToUse(modelconfigAPIClient, userSuppliedBase, resolvedOrigin.Base, supportedBases)
	if err != nil {
		c.Warnf("failed to get a suggested operating system from resolved charm response", map[string]interface{}{"err": err})
	}
	// Double check we got what was requested.
	if !userSuppliedBase.Empty() && !userSuppliedBase.IsCompatible(baseToUse) {
		return nil, jujuerrors.Errorf(
			"juju bug (LP 2039179), requested base %q does not match base %q found for charm.",
			userSuppliedBase, baseToUse)
	}
	resolvedOrigin.Base = baseToUse

	appConfig := input.Config
	if appConfig == nil {
		appConfig = make(map[string]string)
	}
	appConfig["trust"] = fmt.Sprintf("%v", input.Trust)

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
				return nil, err
			}
			placements = append(placements, appPlacement)
		}
	}

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
	err = retry.Call(retry.CallArgs{
		Func: func() error {
			resultOrigin, err := charmsAPIClient.AddCharm(resolvedURL, resolvedOrigin, false)
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
				URL:    resolvedURL,
				Origin: resultOrigin,
			}

			resources, err := c.processResources(charmsAPIClient, conn, charmID, appName)
			if err != nil && !jujuerrors.Is(err, jujuerrors.AlreadyExists) {
				return err
			}

			args := apiapplication.DeployArgs{
				CharmID:         charmID,
				ApplicationName: appName,
				NumUnits:        input.Units,
				CharmOrigin:     resultOrigin,
				Config:          appConfig,
				Cons:            input.Constraints,
				Resources:       resources,
				Placement:       placements,
			}
			c.Tracef("Calling Deploy", map[string]interface{}{"args": args})
			if err = applicationAPIClient.Deploy(args); err != nil {
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
			return !errors.Is(err, jujuerrors.NotFound) && !errors.Is(err, jujuerrors.AlreadyExists)
		},
		NotifyFunc: func(err error, attempt int) {
			c.Errorf(err, fmt.Sprintf("deploy application %q retry", appName))
			message := fmt.Sprintf("waiting for application %q deploy, attempt %d", appName, attempt)
			c.Debugf(message)
		},
		BackoffFunc: retry.DoubleDelay,
		Attempts:    30,
		Delay:       time.Second,
		Clock:       clock.WallClock,
		Stop:        ctx.Done(),
	})
	if err != nil {
		return nil, err
	}

	// If we have managed to deploy something, now we have
	// to check if we have to expose something
	err = c.processExpose(applicationAPIClient, appName, input.Expose)
	return &CreateApplicationResponse{
		AppName: appName,
	}, err
}

// supportedWorkloadBase returns a slice of supported workload basees
// depending on the controller agent version. This provider currently
// uses juju 3.3.0 code. However, the supported workload base list is
// different between juju 2 and juju 3. Handle that here.
func (c applicationsClient) supportedWorkloadBase(imageStream string) ([]base.Base, error) {
	supportedBases, err := base.WorkloadBases(time.Now(), base.Base{}, imageStream)
	if err != nil {
		return nil, err
	}
	if c.controllerVersion.Major > 2 {
		// SupportedBases include those supported with juju 3.x; juju 2.9.x
		// supports more. If we have a juju 2.9.x controller add them back.
		additionallySupported := []base.Base{
			{OS: "ubuntu", Channel: base.Channel{Track: "18.04"}}, // bionic
			{OS: "ubuntu", Channel: base.Channel{Track: "16.04"}}, // xenial
			{OS: "ubuntu", Channel: base.Channel{Track: "14.04"}}, // trusty
			{OS: "ubuntu", Channel: base.Channel{Track: "12.04"}}, // precise
			{OS: "windows"},
			{OS: "centos", Channel: base.Channel{Track: "7"}}, // centos7
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
func (c applicationsClient) baseToUse(modelconfigAPIClient *apimodelconfig.Client, inputBase, suggestedBase base.Base, charmBases []base.Base) (base.Base, error) {
	c.Tracef("baseToUse", map[string]interface{}{"inputBase": inputBase, "suggestedBase": suggestedBase, "charmBases": charmBases})

	attrs, err := modelconfigAPIClient.ModelGet()
	if err != nil {
		return base.Base{}, jujuerrors.Wrap(err, errors.New("cannot fetch model settings"))
	}
	modelConfig, err := config.New(config.NoDefaults, attrs)
	if err != nil {
		return base.Base{}, err
	}

	supportedWorkloadBases, err := c.supportedWorkloadBase(modelConfig.ImageStream())
	if err != nil {
		return base.Base{}, err
	}

	// We can choose from a list of bases, supported both as
	// workload bases and by the charm.
	supportedBases := intersectionOfBases(charmBases, supportedWorkloadBases)
	if len(supportedBases) == 0 {
		return base.Base{}, jujuerrors.NewNotSupported(nil,
			"This charm has no bases supported by the charm and in the list of juju workload bases for the current version of juju.")
	}

	// If the inputBase is supported by the charm and is a supported
	// workload base, use that.
	if basesContain(inputBase, supportedBases) {
		return inputBase, nil
	} else if !inputBase.Empty() {
		return base.Base{}, jujuerrors.NewNotSupported(nil,
			fmt.Sprintf("base %q either not supported by the charm, or an unsupported juju workload base with the current version of juju.", inputBase))
	}

	// If a default base is explicitly defined for the model,
	// use that if a supportedBase.
	defaultBaseString, explicit := modelConfig.DefaultBase()
	if explicit {
		defaultBase, err := base.ParseBaseFromString(defaultBaseString)
		if err != nil {
			return base.Base{}, err
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

// processExpose is a local function that executes an expose request.
// If the exposeConfig argument is nil it simply exits. If not,
// an expose request is done populating the request arguments with
// the endpoints, spaces, and cidrs contained in the exposeConfig
// map.
func (c applicationsClient) processExpose(applicationAPIClient *apiapplication.Client, applicationName string, expose map[string]interface{}) error {
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
		return applicationAPIClient.Expose(applicationName, nil)
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

	return applicationAPIClient.Expose(applicationName, requestParams)
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
func (c applicationsClient) processResources(charmsAPIClient *apicharms.Client, conn api.Connection, charmID apiapplication.CharmID, appName string) (map[string]string, error) {
	charmInfo, err := charmsAPIClient.CharmInfo(charmID.URL.String())
	if err != nil {
		return nil, typedError(err)
	}

	// check if we have resources to request
	if len(charmInfo.Meta.Resources) == 0 {
		return nil, nil
	}

	resourcesAPIClient, err := apiresources.NewClient(conn)
	if err != nil {
		return nil, err
	}

	return addPendingResources(appName, charmInfo.Meta.Resources, charmID, resourcesAPIClient)
}

// ReadApplicationWithRetryOnNotFound calls ReadApplication until
// successful, or the count is exceeded when the error is of type
// not found. Delay indicates how long to wait between attempts.
func (c applicationsClient) ReadApplicationWithRetryOnNotFound(ctx context.Context, input *ReadApplicationInput) (*ReadApplicationResponse, error) {
	var output *ReadApplicationResponse
	err := retry.Call(retry.CallArgs{
		Func: func() error {
			var err error
			output, err = c.ReadApplication(input)
			if errors.As(err, &ApplicationNotFoundError) {
				return nil
			}
			return err
		},
		NotifyFunc: func(err error, attempt int) {
			if attempt%4 == 0 {
				message := fmt.Sprintf("waiting for application %q", input.AppName)
				if attempt != 4 {
					message = "still " + message
				}
				c.Debugf(message)
			}
		},
		BackoffFunc: retry.DoubleDelay,
		Attempts:    30,
		Delay:       time.Second,
		Clock:       clock.WallClock,
		Stop:        ctx.Done(),
	})
	return output, err
}

func (c applicationsClient) ReadApplication(input *ReadApplicationInput) (*ReadApplicationResponse, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	applicationAPIClient := apiapplication.NewClient(conn)
	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	apps, err := applicationAPIClient.ApplicationsInfo([]names.ApplicationTag{names.NewApplicationTag(input.AppName)})
	if err != nil {
		c.Errorf(err, "found when querying the applications info")
		return nil, err
	}
	if len(apps) > 1 {
		return nil, fmt.Errorf("more than one result for application: %s", input.AppName)
	}
	if len(apps) < 1 {
		return nil, fmt.Errorf("no results for application: %s", input.AppName)
	}
	if apps[0].Error != nil {
		return nil, &applicationNotFoundError{input.AppName}
	}

	appInfo := apps[0].Result

	var appConstraints constraints.Value = constraints.Value{}
	// constraints do not apply to subordinate applications.
	if appInfo.Principal {
		queryConstraints, err := applicationAPIClient.GetConstraints(input.AppName)
		if err != nil {
			c.Errorf(err, "found when querying the application constraints")
			return nil, err
		}
		if len(queryConstraints) != 1 {
			return nil, fmt.Errorf("expected one set of application constraints, received %d", len(queryConstraints))
		}
		appConstraints = queryConstraints[0]
	}

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}
	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[input.AppName]; !exists {
		return nil, fmt.Errorf("no status returned for application: %s", input.AppName)
	}

	allocatedMachines := set.NewStrings()
	for _, v := range appStatus.Units {
		if v.Machine != "" {
			allocatedMachines.Add(v.Machine)
		}
	}

	var placement string
	if !allocatedMachines.IsEmpty() {
		placement = strings.Join(allocatedMachines.SortedValues(), ",")
	}

	unitCount := len(appStatus.Units)
	// if we have a CAAS we use scale instead of units length
	modelType, err := c.ModelType(input.ModelName)
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

	returnedConf, err := applicationAPIClient.Get(model.GenerationMaster, input.AppName)
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
		endpoints := []string{""}
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
		exposed["endpoints"] = strings.Join(endpoints, ",")
		exposed["spaces"] = spaces
		exposed["cidrs"] = cidrs
	}
	// ParseChannel to send back a base without the risk.
	// Having the risk will cause issues with the provider
	// saving a different value than the user did.
	baseChannel, err := base.ParseChannel(appInfo.Base.Channel)
	if err != nil {
		return nil, jujuerrors.Annotate(err, "failed parse channel for base")
	}
	seriesString, err := base.GetSeriesFromChannel(appInfo.Base.Name, baseChannel.Track)
	if err != nil {
		return nil, jujuerrors.Annotate(err, "failed to get series from base")
	}
	response := &ReadApplicationResponse{
		Name:        charmURL.Name,
		Channel:     appInfo.Channel,
		Revision:    charmURL.Revision,
		Base:        fmt.Sprintf("%s@%s", appInfo.Base.Name, baseChannel.Track),
		Series:      seriesString,
		Units:       unitCount,
		Trust:       trustValue,
		Expose:      exposed,
		Config:      conf,
		Constraints: appConstraints,
		Principal:   appInfo.Principal,
		Placement:   placement,
	}

	return response, nil
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

func (c applicationsClient) UpdateApplication(input *UpdateApplicationInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	applicationAPIClient := apiapplication.NewClient(conn)
	charmsAPIClient := apicharms.NewClient(conn)
	clientAPIClient := apiclient.NewClient(conn, c.JujuLogger())

	resourcesAPIClient, err := apiresources.NewClient(conn)
	if err != nil {
		return err
	}

	status, err := clientAPIClient.Status(nil)
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

	if auxConfig != nil {
		err := applicationAPIClient.SetConfig("master", input.AppName, "", auxConfig)
		if err != nil {
			c.Errorf(err, "setting configuration params")
			return err
		}
	}

	// unexpose corresponding endpoints
	if len(input.Unexpose) != 0 {
		c.Tracef("Unexposing endpoints", map[string]interface{}{"endpoints": input.Unexpose})
		if err := applicationAPIClient.Unexpose(input.AppName, input.Unexpose); err != nil {
			c.Errorf(err, "when trying to unexpose")
			return err
		}
	}
	// expose endpoints if required
	if input.Expose != nil {
		c.Tracef("Expose endpoints", map[string]interface{}{"endpoints": input.Unexpose})
		err := c.processExpose(applicationAPIClient, input.AppName, input.Expose)
		if err != nil {
			c.Errorf(err, "when trying to expose")
			return err
		}
	}

	if input.Constraints != nil {
		err := applicationAPIClient.SetConstraints(input.AppName, *input.Constraints)
		if err != nil {
			c.Errorf(err, "setting application constraints")
			return err
		}
	}

	if input.Units != nil {
		// TODO: Refactor this to a separate function
		modelType, err := c.ModelType(input.ModelName)
		if err != nil {
			return err
		}
		if modelType == model.CAAS {
			_, err := applicationAPIClient.ScaleApplication(apiapplication.ScaleApplicationParams{
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
				_, err := applicationAPIClient.AddUnits(apiapplication.AddUnitsParams{
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
				_, err := applicationAPIClient.DestroyUnits(apiapplication.DestroyUnitsParams{
					Units:          unitsToDestroy,
					DestroyStorage: true,
				})
				if err != nil {
					return err
				}
			}
		}
	}

	// Use the revision and channel info to create the
	// corresponding SetCharm info.
	if input.Revision != nil || input.Channel != "" {
		setCharmConfig, err := c.computeSetCharmConfig(input, applicationAPIClient, charmsAPIClient, resourcesAPIClient)
		if err != nil {
			return err
		}

		err = applicationAPIClient.SetCharm(model.GenerationMaster, *setCharmConfig)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c applicationsClient) DestroyApplication(input *DestroyApplicationInput) error {
	conn, err := c.GetConnection(&input.ModelName)
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

	_, err = applicationAPIClient.DestroyApplications(destroyParams)

	if err != nil {
		return err
	}

	return nil
}

// computeSetCharmConfig populates the corresponding configuration object
// to indicate juju what charm to be deployed.
func (c applicationsClient) computeSetCharmConfig(
	input *UpdateApplicationInput,
	applicationAPIClient *apiapplication.Client,
	charmsAPIClient *apicharms.Client,
	resourcesAPIClient *apiresources.Client,
) (*apiapplication.SetCharmConfig, error) {
	oldURL, oldOrigin, err := applicationAPIClient.GetCharmURLOrigin("", input.AppName)
	if err != nil {
		return nil, err
	}

	newURL := oldURL
	if input.Revision != nil {
		newURL = oldURL.WithRevision(*input.Revision)
	}

	newOrigin := oldOrigin
	if input.Channel != "" {
		parsedChannel, err := charm.ParseChannel(input.Channel)
		if err != nil {
			return nil, err
		}
		if parsedChannel.Track != "" {
			newOrigin.Track = strPtr(parsedChannel.Track)
		}
		newOrigin.Risk = string(parsedChannel.Risk)
		if parsedChannel.Branch != "" {
			newOrigin.Branch = strPtr(parsedChannel.Branch)
		}
	}

	resolvedURL, resolvedOrigin, _, err := resolveCharm(charmsAPIClient, newURL, newOrigin)
	if err != nil {
		return nil, err
	}

	resultOrigin, err := charmsAPIClient.AddCharm(resolvedURL, resolvedOrigin, false)
	if err != nil {
		return nil, err
	}

	apiCharmID := apiapplication.CharmID{
		URL:    newURL,
		Origin: resultOrigin,
	}

	resourceIDs, err := c.updateResources(input.AppName, charmsAPIClient, apiCharmID, resourcesAPIClient)
	if err != nil {
		return nil, err
	}

	toReturn := apiapplication.SetCharmConfig{
		ApplicationName: input.AppName,
		CharmID:         apiCharmID,
		ResourceIDs:     resourceIDs,
	}

	return &toReturn, nil
}

func resolveCharm(charmsAPIClient *apicharms.Client, curl *charm.URL, origin apicommoncharm.Origin) (*charm.URL, apicommoncharm.Origin, []base.Base, error) {
	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolved, err := charmsAPIClient.ResolveCharms([]apicharms.CharmToResolve{{URL: curl, Origin: origin}})
	if err != nil {
		return nil, apicommoncharm.Origin{}, []base.Base{}, err
	}
	if len(resolved) != 1 {
		return nil, apicommoncharm.Origin{}, []base.Base{}, fmt.Errorf("expected only one resolution, received %d", len(resolved))
	}
	resolvedCharm := resolved[0]
	return resolvedCharm.URL, resolvedCharm.Origin, resolvedCharm.SupportedBases, resolvedCharm.Error
}

func strPtr(in string) *string {
	return &in
}

func (c applicationsClient) updateResources(appName string, charmsAPIClient *apicharms.Client,
	charmID apiapplication.CharmID, resourcesAPIClient *apiresources.Client) (map[string]string, error) {
	meta, err := utils.GetMetaResources(charmID.URL, charmsAPIClient)
	if err != nil {
		return nil, err
	}
	// TODO (cderici): Provided resources for GetUpgradeResources are user inputs.
	// It's a map[string]string that should come from the plan itself. We currently
	// don't have a resources block in the charm.
	filtered, err := utils.GetUpgradeResources(
		charmID,
		charmsAPIClient,
		resourcesAPIClient,
		appName,
		nil,
		meta,
	)
	if err != nil {
		return nil, err
	}
	if len(filtered) == 0 {
		return nil, nil
	}

	return addPendingResources(appName, filtered, charmID, resourcesAPIClient)
}

func addPendingResources(appName string, resourcesToBeAdded map[string]charmresources.Meta,
	charmID apiapplication.CharmID, resourcesAPIClient *apiresources.Client) (map[string]string, error) {
	pendingResources := []charmresources.Resource{}
	for _, v := range resourcesToBeAdded {
		aux := charmresources.Resource{
			Meta:     v,
			Origin:   charmresources.OriginStore,
			Revision: -1,
		}
		pendingResources = append(pendingResources, aux)
	}

	resourcesReq := apiresources.AddPendingResourcesArgs{
		ApplicationID: appName,
		CharmID: apiresources.CharmID{
			URL:    charmID.URL,
			Origin: charmID.Origin,
		},
		Resources: pendingResources,
	}

	toRequest, err := resourcesAPIClient.AddPendingResources(resourcesReq)
	if err != nil {
		return nil, typedError(err)
	}

	// now build a map with the resource name and the corresponding UUID
	toReturn := map[string]string{}
	for i, argsResource := range pendingResources {
		toReturn[argsResource.Meta.Name] = toRequest[i]
	}

	return toReturn, nil
}
