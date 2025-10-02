// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"bytes"

	charmresources "github.com/juju/charm/v12/resource"
	jujuerrors "github.com/juju/errors"
	apiapplication "github.com/juju/juju/api/client/application"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/docker"
	"gopkg.in/yaml.v3"
)

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

// Equal checks if two CharmResource instances are equal.
func (cr CharmResource) Equal(in CharmResource) bool {
	if cr.RevisionNumber != in.RevisionNumber {
		return false
	}
	if cr.OCIImageURL != in.OCIImageURL {
		return false
	}
	if cr.RegistryUser != in.RegistryUser {
		return false
	}
	if cr.RegistryPassword != in.RegistryPassword {
		return false
	}
	return true
}

type CharmResources map[string]CharmResource

// Equal checks if two CharmResources maps are equal.
func (cr CharmResources) Equal(other CharmResources) bool {
	// Both nil
	if cr == nil && other == nil {
		return true
	}
	// Since both are not nil, if either is nil they are not equal.
	if cr == nil || other == nil {
		return false
	}
	// Different lengths, not equal
	if len(cr) != len(other) {
		return false
	}
	// Compare each key/value pair
	for k, v := range cr {
		ov, found := other[k]
		if !found {
			return false
		}
		if !v.Equal(ov) {
			return false
		}
	}
	return true
}

type charmResourceReadSeeker struct {
	*bytes.Reader
}

// ToResourceReader converts the CharmResource to a reader that can be used
// to upload the resource to Juju. It returns an error if the conversion fails.
func (cr CharmResource) ToResourceReader() (charmResourceReadSeeker, error) {
	if cr.OCIImageURL == "" {
		return charmResourceReadSeeker{}, jujuerrors.New("OCIImageURL is required to create a resource reader")
	}

	registryDetails := resources.DockerImageDetails{
		RegistryPath: cr.OCIImageURL,
		ImageRepoDetails: docker.ImageRepoDetails{
			BasicAuthConfig: docker.BasicAuthConfig{
				Username: cr.RegistryUser,
				Password: cr.RegistryPassword,
			},
		},
	}
	details, err := yaml.Marshal(registryDetails)
	if err != nil {
		return charmResourceReadSeeker{}, err
	}
	return charmResourceReadSeeker{bytes.NewReader([]byte(details))}, nil
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
		if t != charmresources.TypeContainerImage {
			// Non-docker resources are not supported for local upload.
			return jujuerrors.NotSupportedf("uploading local resource of type %v for resource %v",
				t, pendingResUpload.Name)
		}

		localResource, ok := charmResources[pendingResUpload.Name]
		if !ok {
			return jujuerrors.NotFoundf("resource %v not found in input resources", pendingResUpload.Name)
		}

		r, err := localResource.ToResourceReader()
		if err != nil {
			return jujuerrors.Trace(err)
		}
		uploadErr := resourceAPIClient.Upload(appName, pendingResUpload.Name, pendingResUpload.Filename, "", r)
		if uploadErr != nil {
			return jujuerrors.Trace(uploadErr)
		}
	}
	return nil
}
