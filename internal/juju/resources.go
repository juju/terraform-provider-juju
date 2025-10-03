// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"bytes"
	"maps"

	charmresources "github.com/juju/charm/v12/resource"
	jujuerrors "github.com/juju/errors"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/docker"
	"gopkg.in/yaml.v3"
)

// CharmResource represents a resource associated with a charm.
type CharmResource struct {
	RevisionNumber   string
	OCIImageURL      string
	RegistryUser     string
	RegistryPassword string
}

// String returns a string representation of the CharmResource.
// The string is a valid resource representation for the Juju API.
// A revision number indicates to Juju that the resource will come
// from Charmhub while any non-integer indicates that the client
// must upload the resource.
func (cr CharmResource) String() string {
	if cr.RevisionNumber != "" {
		return cr.RevisionNumber
	}
	return cr.OCIImageURL
}

// CharmResources is a map of resource names to CharmResource instances.
type CharmResources map[string]CharmResource

// Equal checks if two CharmResources maps are equal.
func (cr CharmResources) Equal(other CharmResources) bool {
	return maps.Equal(cr, other)
}

// MarhsalYaml marshals the CharmResource into a YAML representation
// suitable for uploading to Juju as a resource.
func (cr CharmResource) MarhsalYaml() ([]byte, error) {
	registryDetails := resources.DockerImageDetails{
		RegistryPath: cr.OCIImageURL,
		ImageRepoDetails: docker.ImageRepoDetails{
			BasicAuthConfig: docker.BasicAuthConfig{
				Username: cr.RegistryUser,
				Password: cr.RegistryPassword,
			},
		},
	}
	return yaml.Marshal(registryDetails)
}

// UploadExistingPendingResources uploads local resources. Used
// after DeployFromRepository, where the resources have been added
// to the controller.
func uploadExistingPendingResources(
	appName string,
	pendingResources []apiapplication.PendingResourceUpload,
	charmResources map[string]CharmResource,
	resourceAPIClient ResourceAPIClient) error {
	if pendingResources == nil {
		return nil
	}

	for _, pendingResUpload := range pendingResources {
		t, typeParseErr := charmresources.ParseType(pendingResUpload.Type)
		if typeParseErr != nil {
			return jujuerrors.Annotatef(typeParseErr, "invalid type %v for pending resource %v",
				pendingResUpload.Type, pendingResUpload.Name)
		}
		if t != charmresources.TypeContainerImage { // Uploading a container image implies uploading image metadata.
			// Non-docker resources are not supported for local upload.
			return jujuerrors.NotSupportedf("uploading local resource of type %v for resource %v",
				t, pendingResUpload.Name)
		}

		localResource, ok := charmResources[pendingResUpload.Name]
		if !ok {
			return jujuerrors.NotFoundf("resource %v not found in input resources", pendingResUpload.Name)
		}
		details, err := localResource.MarhsalYaml()
		if err != nil {
			return jujuerrors.Trace(err)
		}
		uploadErr := resourceAPIClient.Upload(appName, pendingResUpload.Name, pendingResUpload.Filename, "", bytes.NewReader(details))
		if uploadErr != nil {
			return jujuerrors.Trace(uploadErr)
		}
	}
	return nil
}
