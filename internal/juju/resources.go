// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"strconv"

	"github.com/juju/charm/v12"
	charmresources "github.com/juju/charm/v12/resource"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apimodelconfig "github.com/juju/juju/api/client/modelconfig"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/cmd/juju/application/utils"
	resourcecmd "github.com/juju/juju/cmd/juju/resource"
	corebase "github.com/juju/juju/core/base"
)

// isInt checks if strings could be converted to an integer
// Used to detect resources which are given with revision number
func isInt(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}

func (c applicationsClient) getResourceIDs(transformedInput transformedCreateApplicationInput, conn api.Connection, deployInfo apiapplication.DeployInfo, pendingResources []apiapplication.PendingResourceUpload) (map[string]string, error) {
	resourceIDs := map[string]string{}
	charmsAPIClient := apicharms.NewClient(conn)
	modelconfigAPIClient := apimodelconfig.NewClient(conn)
	resourcesAPIClient, err := c.getResourceAPIClient(conn)
	if err != nil {
		return resourceIDs, err
	}
	resolvedURL, resolvedOrigin, supportedBases, err := getCharmResolvedUrlAndOrigin(conn, transformedInput)
	if err != nil {
		return resourceIDs, err
	}
	userSuppliedBase := transformedInput.charmBase
	baseToUse, err := c.baseToUse(modelconfigAPIClient, userSuppliedBase, resolvedOrigin.Base, supportedBases)
	if err != nil {
		return resourceIDs, err
	}

	resolvedOrigin.Base = baseToUse

	// 3.3 version of ResolveCharm does not always include the series
	// in the url. However, juju 2.9 requires it.
	series, err := corebase.GetSeriesFromBase(baseToUse)
	if err != nil {
		return resourceIDs, err
	}
	resolvedURL = resolvedURL.WithSeries(series)

	resultOrigin, err := charmsAPIClient.AddCharm(resolvedURL, resolvedOrigin, false)
	if err != nil {
		return resourceIDs, err
	}
	charmID := apiapplication.CharmID{
		URL:    resolvedURL.String(),
		Origin: resultOrigin,
	}

	charmInfo, err := charmsAPIClient.CharmInfo(charmID.URL)
	if err != nil {
		return resourceIDs, err
	}

	for _, resourceMeta := range charmInfo.Meta.Resources {
		for _, pendingResource := range pendingResources {
			if pendingResource.Name == resourceMeta.Name {
				fileSystem := osFilesystem{}
				localResource := charmresources.Resource{
					Meta:   resourceMeta,
					Origin: charmresources.OriginStore,
				}
				t, typeParseErr := charmresources.ParseType(resourceMeta.Type.String())
				if typeParseErr != nil {
					return resourceIDs, typeParseErr
				}
				r, openResErr := resourcecmd.OpenResource(pendingResource.Filename, t, fileSystem.Open)
				if openResErr != nil {
					return resourceIDs, openResErr
				}
				toRequestUpload, err := resourcesAPIClient.UploadPendingResource(deployInfo.Name, localResource, pendingResource.Filename, r)
				if err != nil {
					return resourceIDs, err
				}
				resourceIDs[resourceMeta.Name] = toRequestUpload
			}
		}
	}
	return resourceIDs, nil
}

func getCharmResolvedUrlAndOrigin(conn api.Connection, transformedInput transformedCreateApplicationInput) (*charm.URL, apicommoncharm.Origin, []corebase.Base, error) {
	charmsAPIClient := apicharms.NewClient(conn)
	modelconfigAPIClient := apimodelconfig.NewClient(conn)

	channel, err := charm.ParseChannel(transformedInput.charmChannel)
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}

	charmURL, err := resolveCharmURL(transformedInput.charmName)
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}

	if charmURL.Revision != UnspecifiedRevision {
		err := fmt.Errorf("cannot specify revision in a charm name")
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}
	if transformedInput.charmRevision != UnspecifiedRevision && channel.Empty() {
		err = fmt.Errorf("specifying a revision requires a channel for future upgrades")
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}

	userSuppliedBase := transformedInput.charmBase
	platformCons, err := modelconfigAPIClient.GetModelConstraints()
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}
	platform := utils.MakePlatform(transformedInput.constraints, userSuppliedBase, platformCons)

	urlForOrigin := charmURL
	if transformedInput.charmRevision != UnspecifiedRevision {
		urlForOrigin = urlForOrigin.WithRevision(transformedInput.charmRevision)
	}

	// Juju 2.9 cares that the series is in the origin. Juju 3.3 does not.
	// We are supporting both now.
	if !userSuppliedBase.Empty() {
		userSuppliedSeries, err := corebase.GetSeriesFromBase(userSuppliedBase)
		if err != nil {
			return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
		}
		urlForOrigin = urlForOrigin.WithSeries(userSuppliedSeries)
	}

	origin, err := utils.MakeOrigin(charm.Schema(urlForOrigin.Schema), transformedInput.charmRevision, channel, platform)
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}

	// Charm or bundle has been supplied as a URL, so we resolve and
	// deploy using the store but pass in the origin command line
	// argument so users can target a specific origin.
	resolvedURL, resolvedOrigin, supportedBases, err := resolveCharm(charmsAPIClient, charmURL, origin)
	if err != nil {
		return nil, apicommoncharm.Origin{}, []corebase.Base{}, err
	}

	return resolvedURL, resolvedOrigin, supportedBases, nil
}
