// Package juju is a facade to make interacting with Juju clients simpler. It also acts as an insulating layer
// protecting the provider package from upstream changes.
// The long-term intention is for this package to be removed. Eventually, it would be nice for this package to
// be replaced with more granular clients in Juju itself. Note that much of this code is duplicated from Juju CLI
// commands.
package juju

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/juju/juju/core/model"
	"github.com/juju/juju/rpc/params"
	"github.com/rs/zerolog/log"

	"github.com/juju/charm/v8"
	charmresources "github.com/juju/charm/v8/resource"
	jujuerrors "github.com/juju/errors"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apiclient "github.com/juju/juju/api/client/client"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	apiresources "github.com/juju/juju/api/client/resources"
	"github.com/juju/juju/cmd/juju/application/utils"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/version"
	"github.com/juju/names/v4"
)

type applicationsClient struct {
	ConnectionFactory
}

type CreateApplicationInput struct {
	ApplicationName string
	ModelUUID       string
	CharmName       string
	CharmChannel    string
	CharmSeries     string
	CharmRevision   int
	Units           int
	Trust           bool
	Expose          map[string]interface{}
	Config          map[string]string
}

type CreateApplicationResponse struct {
	AppName  string
	Revision int
	Series   string
}

type ReadApplicationInput struct {
	ModelUUID string
	AppName   string
}

type ReadApplicationResponse struct {
	Name     string
	Channel  string
	Revision int
	Series   string
	Units    int
	Trust    bool
	Config   map[string]string
	Expose   map[string]interface{}
}

type UpdateApplicationInput struct {
	ModelUUID string
	ModelType string
	AppName   string
	//Channel   string // TODO: Unsupported for now
	Units    *int
	Revision *int
	Trust    *bool
	Expose   map[string]interface{}
	// Unexpose indicates what endpoints to unexpose
	Unexpose []string
	Config   map[string]string
	//Series    string // TODO: Unsupported for now
}

type DestroyApplicationInput struct {
	ApplicationName string
	ModelUUID       string
}

func newApplicationClient(cf ConnectionFactory) *applicationsClient {
	return &applicationsClient{
		ConnectionFactory: cf,
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

func (c applicationsClient) CreateApplication(input *CreateApplicationInput) (*CreateApplicationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	charmsAPIClient := apicharms.NewClient(conn)
	defer charmsAPIClient.Close()

	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

	applicationAPIClient := apiapplication.NewClient(conn)
	defer applicationAPIClient.Close()

	modelconfigAPIClient := apimodelconfig.NewClient(conn)
	defer modelconfigAPIClient.Close()

	resourcesAPIClient, err := apiresources.NewClient(conn)
	if err != nil {
		return nil, err
	}

	defer resourcesAPIClient.Close()

	appName := input.ApplicationName
	if appName == "" {
		appName = input.CharmName
	}
	if err := names.ValidateApplicationName(appName); err != nil {
		return nil, err
	}

	channel, err := charm.ParseChannel(input.CharmChannel)
	if err != nil {
		return nil, err
	}

	charmURL, err := resolveCharmURL(input.CharmName)
	if err != nil {
		return nil, err
	}

	if charmURL.Revision != UnspecifiedRevision {
		return nil, fmt.Errorf("cannot specify revision in a charm or bundle name")
	}
	if input.CharmRevision != UnspecifiedRevision && channel.Empty() {
		return nil, fmt.Errorf("specifying a revision requires a channel for future upgrades")
	}

	modelConstraints, err := clientAPIClient.GetModelConstraints()
	if err != nil {
		return nil, err
	}
	platform, err := utils.DeducePlatform(constraints.Value{}, input.CharmSeries, modelConstraints)
	if err != nil {
		return nil, err
	}
	urlForOrigin := charmURL
	if input.CharmRevision != UnspecifiedRevision {
		urlForOrigin = urlForOrigin.WithRevision(input.CharmRevision)
	}
	origin, err := utils.DeduceOrigin(urlForOrigin, channel, platform)
	if err != nil {
		return nil, err
	}
	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolved, err := charmsAPIClient.ResolveCharms([]apicharms.CharmToResolve{{URL: charmURL, Origin: origin}})
	if err != nil {
		return nil, err
	}
	if len(resolved) != 1 {
		return nil, fmt.Errorf("expected only one resolution, received %d", len(resolved))
	}
	resolvedCharm := resolved[0]

	if resolvedCharm.Error != nil {
		return nil, resolvedCharm.Error
	}

	// Figure out the actual series of the charm
	var series string
	switch {
	case input.CharmSeries != "":
		// Explicitly request series.
		series = input.CharmSeries
	case charmURL.Series != "":
		// Series specified in charm URL.
		series = charmURL.Series
	default:
		// First try using the default model series if explicitly set, provided
		// it is supported by the charm.
		// Get the model config
		attrs, err := modelconfigAPIClient.ModelGet()
		if err != nil {
			return nil, jujuerrors.Wrap(err, errors.New("cannot fetch model settings"))
		}
		modelConfig, err := config.New(config.NoDefaults, attrs)
		if err != nil {
			return nil, err
		}

		var explicit bool
		series, explicit = modelConfig.DefaultSeries()
		if explicit {
			_, err := charm.SeriesForCharm(series, resolvedCharm.SupportedSeries)
			if err == nil {
				break
			}
		}

		// Finally, because we are forced we choose LTS
		series = version.DefaultSupportedLTS()
	}

	// Select an actually supported series
	series, err = charm.SeriesForCharm(series, resolvedCharm.SupportedSeries)
	if err != nil {
		return nil, err
	}

	// Add the charm to the model
	origin = resolvedCharm.Origin.WithSeries(series)

	var deployRevision int
	if input.CharmRevision > -1 {
		deployRevision = input.CharmRevision
	} else {
		if origin.Revision != nil {
			deployRevision = *origin.Revision
		} else {
			return nil, errors.New("no origin revision")
		}
	}

	charmURL = resolvedCharm.URL.WithRevision(deployRevision).WithArchitecture(origin.Architecture).WithSeries(series)
	resultOrigin, err := charmsAPIClient.AddCharm(charmURL, origin, false)
	if err != nil {
		return nil, err
	}

	charmID := apiapplication.CharmID{
		URL:    charmURL,
		Origin: resultOrigin,
	}

	resources, err := c.processResources(charmsAPIClient, resourcesAPIClient, charmID, input.ApplicationName)
	if err != nil {
		return nil, err
	}

	// TODO: This should probably be set within the schema
	// For now this is the default required behaviour
	var appConfig map[string]string
	if input.Config != nil {
		appConfig = input.Config
	} else {
		appConfig = make(map[string]string)
	}

	appConfig["trust"] = fmt.Sprintf("%v", input.Trust)

	err = applicationAPIClient.Deploy(apiapplication.DeployArgs{
		CharmID:         charmID,
		ApplicationName: appName,
		NumUnits:        input.Units,
		Series:          resultOrigin.Series,
		CharmOrigin:     resultOrigin,
		Config:          appConfig,
		Resources:       resources,
	})

	if err != nil {
		// unfortunate error during deployment
		return &CreateApplicationResponse{
			AppName:  appName,
			Revision: *origin.Revision,
			Series:   series,
		}, err
	}

	// If we have managed to deploy something, now we have
	// to check if we have to expose something
	err = c.processExpose(applicationAPIClient, input.ApplicationName, input.Expose)
	return &CreateApplicationResponse{
		AppName:  appName,
		Revision: *origin.Revision,
		Series:   series,
	}, err
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
		log.Trace().Msgf("call expose application [%s]", applicationName)
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

	log.Trace().Interface("ExposeParams", requestParams).Msg("call expose API endpoint")

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
func (c applicationsClient) processResources(charmsAPIClient *apicharms.Client, resourcesAPIClient *apiresources.Client, charmID apiapplication.CharmID, appName string) (map[string]string, error) {
	charmInfo, err := charmsAPIClient.CharmInfo(charmID.URL.String())
	if err != nil {
		return nil, err
	}

	// check if we have resources to request
	if len(charmInfo.Meta.Resources) == 0 {
		return nil, nil
	}

	pendingResources := []charmresources.Resource{}
	for _, v := range charmInfo.Meta.Resources {
		aux := charmresources.Resource{
			Meta: charmresources.Meta{
				Name:        v.Name,
				Type:        v.Type,
				Path:        v.Path,
				Description: v.Description,
			},
			Origin: charmresources.OriginStore,
			// TODO: prepare for resources with different versions
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
		CharmStoreMacaroon: nil,
		Resources:          pendingResources,
	}

	toRequest, err := resourcesAPIClient.AddPendingResources(resourcesReq)
	if err != nil {
		return nil, err
	}

	// now build a map with the resource name and the corresponding UUID
	toReturn := map[string]string{}
	for i, argsResource := range pendingResources {
		toReturn[argsResource.Meta.Name] = toRequest[i]
	}

	return toReturn, nil
}

func (c applicationsClient) ReadApplication(input *ReadApplicationInput) (*ReadApplicationResponse, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	applicationAPIClient := apiapplication.NewClient(conn)
	defer applicationAPIClient.Close()

	charmsAPIClient := apicharms.NewClient(conn)
	defer charmsAPIClient.Close()

	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

	apps, err := applicationAPIClient.ApplicationsInfo([]names.ApplicationTag{names.NewApplicationTag(input.AppName)})
	if err != nil {
		return nil, err
	}
	if len(apps) > 1 {
		return nil, fmt.Errorf("more than one result for application: %s", input.AppName)
	}
	if len(apps) < 1 {
		return nil, fmt.Errorf("no results for application: %s", input.AppName)
	}
	appInfo := apps[0].Result

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}
	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[input.AppName]; !exists {
		return nil, fmt.Errorf("no status returned for application: %s", input.AppName)
	}

	unitCount := len(appStatus.Units)

	// NOTE: we are assuming that this charm comes from CharmHub
	charmURL, err := charm.ParseURL(appStatus.Charm)
	if err != nil {
		return nil, fmt.Errorf("failed to parse charm: %v", err)
	}

	returnedConf, err := applicationAPIClient.Get("master", input.AppName)
	if err != nil {
		return nil, fmt.Errorf("failed to get app configuration %v", err)
	}

	conf := make(map[string]string, 0)
	if returnedConf.ApplicationConfig != nil {
		for k, v := range returnedConf.ApplicationConfig {
			// The API returns the configuration entries as interfaces
			// In the terraform plan we introduce strings...
			// so we force this conversion
			aux := v.(map[string]interface{})
			// set if we find the value key.
			// TODO cast the value to the corresponding type
			// indicated in the type field
			if value, found := aux["value"]; found {
				conf[k] = fmt.Sprintf("%s", value)
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

	response := &ReadApplicationResponse{
		Name:     charmURL.Name,
		Channel:  appStatus.CharmChannel,
		Revision: charmURL.Revision,
		Series:   appInfo.Series,
		Units:    unitCount,
		Trust:    trustValue,
		Expose:   exposed,
		Config:   conf,
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
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	applicationAPIClient := apiapplication.NewClient(conn)
	defer applicationAPIClient.Close()

	charmsAPIClient := apicharms.NewClient(conn)
	defer charmsAPIClient.Close()

	clientAPIClient := apiclient.NewClient(conn)
	defer clientAPIClient.Close()

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
	auxConfig := input.Config

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
			log.Error().Err(err).Msg("error setting configuration params")
			return err
		}
	}

	// unexpose corresponding endpoints
	if len(input.Unexpose) != 0 {
		log.Trace().Interface("endpoints", input.Unexpose).Msg("Unexposing endpoints")
		if err := applicationAPIClient.Unexpose(input.AppName, input.Unexpose); err != nil {
			log.Error().Err(err).Msg("error when trying to unexpose")
			return err
		}
	}
	// expose endpoints if required
	if input.Expose != nil {
		log.Trace().Interface("endpoints", input.Unexpose).Msg("Expose endpoints")
		// exposeMap := make(map[string]string)
		// for k, v := range input.Expose {
		// 	exposeMap[k] = v.(string)
		// }
		err := c.processExpose(applicationAPIClient, input.AppName, input.Expose)
		if err != nil {
			log.Error().Err(err).Msg("error when trying to expose")
			return err
		}
	}

	if input.Units != nil {
		// TODO: Refactor this to a separate function
		if input.ModelType == model.CAAS.String() {
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

	if input.Revision != nil {
		// TODO: How do we actually set the revision?
		// It looks like it is set by updating the charmURL which encodes the revision
		oldURL, err := applicationAPIClient.GetCharmURL("", input.AppName)
		if err != nil {
			return err
		}

		newURL := oldURL.WithRevision(*input.Revision)

		modelConstraints, err := clientAPIClient.GetModelConstraints()
		if err != nil {
			return err
		}
		platform, err := utils.DeducePlatform(constraints.Value{}, appStatus.Series, modelConstraints)
		if err != nil {
			return err
		}

		channel, err := charm.ParseChannel(appStatus.CharmChannel)
		if err != nil {
			return err
		}

		origin, err := utils.DeduceOrigin(newURL, channel, platform)
		if err != nil {
			return err
		}

		resultOrigin, err := charmsAPIClient.AddCharm(newURL, origin, false)
		if err != nil {
			return err
		}

		err = applicationAPIClient.SetCharm("", apiapplication.SetCharmConfig{
			ApplicationName: input.AppName,
			CharmID: apiapplication.CharmID{
				URL:    newURL,
				Origin: resultOrigin,
			},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (c applicationsClient) DestroyApplication(input *DestroyApplicationInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	applicationAPIClient := apiapplication.NewClient(conn)
	defer applicationAPIClient.Close()

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
