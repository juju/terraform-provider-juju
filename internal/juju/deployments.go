package juju

import (
	"errors"
	"fmt"

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

func newDeploymentsClient(cf ConnectionFactory) *deploymentsClient {
	return &deploymentsClient{
		ConnectionFactory: cf,
	}
}

func (c deploymentsClient) CreateDeployment(input *CreateDeploymentInput) (string, error) {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return "", err
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
		return "", err
	}

	channel, err := charm.ParseChannel(input.CharmChannel)
	if err != nil {
		return "", err
	}

	path, err := charm.EnsureSchema(input.CharmName, charm.CharmHub)
	if err != nil {
		return "", err
	}
	charmURL, err := charm.ParseURL(path)
	if err != nil {
		return "", err
	}

	if charmURL.Revision != UnspecifiedRevision {
		return "", fmt.Errorf("cannot specify revision in a charm or bundle name")
	}
	if input.CharmRevision != UnspecifiedRevision && channel.Empty() {
		return "", fmt.Errorf("specifying a revision requires a channel for future upgrades")
	}

	modelConstraints, err := clientAPIClient.GetModelConstraints()
	if err != nil {
		return "", err
	}
	platform, err := utils.DeducePlatform(constraints.Value{}, input.CharmSeries, modelConstraints)
	if err != nil {
		return "", err
	}
	urlForOrigin := charmURL
	if input.CharmRevision != UnspecifiedRevision {
		urlForOrigin = urlForOrigin.WithRevision(input.CharmRevision)
	}
	origin, err := utils.DeduceOrigin(urlForOrigin, channel, platform)
	if err != nil {
		return "", err
	}
	// Charm or bundle has been supplied as a URL so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	rev := UnspecifiedRevision
	origin.Revision = &rev
	resolved, err := charmsAPIClient.ResolveCharms([]apicharms.CharmToResolve{{URL: charmURL, Origin: origin}})
	if err != nil {
		return "", err
	}
	if len(resolved) != 1 {
		return "", fmt.Errorf("expected only one resolution, received %d", len(resolved))
	}
	resolvedCharm := resolved[0]

	if err != nil {
		return "", err
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
			return "", jujuerrors.Wrap(err, errors.New("cannot fetch model settings"))
		}
		modelConfig, err := config.New(config.NoDefaults, attrs)
		if err != nil {
			return "", err
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
		return "", err
	}

	// Add the charm to the model
	origin = resolvedCharm.Origin.WithSeries(series)
	charmURL = resolvedCharm.URL.WithRevision(*origin.Revision).WithArchitecture(origin.Architecture).WithSeries(series)
	resultOrigin, err := charmsAPIClient.AddCharm(charmURL, origin, false)
	if err != nil {
		return "", err
	}

	err = applicationAPIClient.Deploy(application.DeployArgs{
		CharmID: application.CharmID{
			URL:    charmURL,
			Origin: resultOrigin,
		},
		ApplicationName: appName,
		NumUnits:        input.Units,
		Series:          resultOrigin.Series,
	})
	return appName, err
}
