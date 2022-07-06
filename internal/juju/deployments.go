package juju

import (
	"errors"
	"fmt"

	"github.com/juju/juju/rpc/params"

	"github.com/juju/charm/v8"
	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api/client/application"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apiclient "github.com/juju/juju/api/client/client"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	"github.com/juju/juju/cmd/juju/application/utils"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/version"
	"github.com/juju/names/v4"
)

type deploymentsClient struct {
	ConnectionFactory
}

type CreateDeploymentInput struct {
	ApplicationName string
	ModelUUID       string
	CharmName       string
	CharmChannel    string
	CharmSeries     string
	CharmRevision   int
	Units           int
}

type DestroyDeploymentInput struct {
	ApplicationName string
	ModelUUID       string
}

type CreateDeploymentResponse struct {
	AppName  string
	Revision int
	Series   string
}

type ReadDeploymentInput struct {
	ModelUUID string
	AppName   string
}

type ReadDeploymentResponse struct {
	Name     string
	Channel  string
	Revision int
	Series   string
	Units    int
	Config   map[string]interface{}
}

func newDeploymentsClient(cf ConnectionFactory) *deploymentsClient {
	return &deploymentsClient{
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

func (c deploymentsClient) CreateDeployment(input *CreateDeploymentInput) (*CreateDeploymentResponse, error) {
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
	rev := UnspecifiedRevision
	origin.Revision = &rev
	resolved, err := charmsAPIClient.ResolveCharms([]apicharms.CharmToResolve{{URL: charmURL, Origin: origin}})
	if err != nil {
		return nil, err
	}
	if len(resolved) != 1 {
		return nil, fmt.Errorf("expected only one resolution, received %d", len(resolved))
	}
	resolvedCharm := resolved[0]

	if err != nil {
		return nil, err
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
		deployRevision = *origin.Revision
	}

	charmURL = resolvedCharm.URL.WithRevision(deployRevision).WithArchitecture(origin.Architecture).WithSeries(series)
	resultOrigin, err := charmsAPIClient.AddCharm(charmURL, origin, false)
	if err != nil {
		return nil, err
	}

	err = applicationAPIClient.Deploy(application.DeployArgs{
		CharmID: application.CharmID{
			URL:    charmURL,
			Origin: resultOrigin,
		},
		ApplicationName: appName,
		NumUnits:        input.Units,
		Series:          resultOrigin.Series,
		CharmOrigin:     resultOrigin,
	})
	return &CreateDeploymentResponse{
		AppName:  appName,
		Revision: *origin.Revision,
		Series:   series,
	}, err
}

func (c deploymentsClient) ReadDeployment(input *ReadDeploymentInput) (*ReadDeploymentResponse, error) {
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
		return nil, errors.New(fmt.Sprintf("more than one result for application: %s", input.AppName))
	}
	if len(apps) < 1 {
		return nil, errors.New(fmt.Sprintf("no results for application: %s", input.AppName))
	}
	appInfo := apps[0].Result

	status, err := clientAPIClient.Status(nil)
	if err != nil {
		return nil, err
	}
	var appStatus params.ApplicationStatus
	var exists bool
	if appStatus, exists = status.Applications[input.AppName]; !exists {
		return nil, errors.New(fmt.Sprintf("no status returned for application: %s", input.AppName))
	}

	unitCount := len(appStatus.Units)

	// NOTE: we are assuming that this is charm comes from CharmHub
	charmURL, err := charm.ParseURL(appStatus.Charm)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to parse charm: %v", err))
	}

	response := &ReadDeploymentResponse{
		Name:     appInfo.Charm,
		Channel:  appStatus.CharmChannel,
		Revision: charmURL.Revision,
		Series:   appInfo.Series,
		Units:    unitCount,
	}

	return response, nil
}

func (c deploymentsClient) DestroyDeployment(input *DestroyDeploymentInput) error {
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
