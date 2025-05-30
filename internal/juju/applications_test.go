// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

// Basic imports
import (
	"context"
	"fmt"
	"testing"

	charmresources "github.com/juju/charm/v12/resource"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/charmhub/transport"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/environs/config"
	"github.com/juju/utils/v3"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type ApplicationSuite struct {
	suite.Suite

	testModelName string

	mockApplicationClient *MockApplicationAPIClient
	mockClient            *MockClientAPIClient
	mockResourceAPIClient *MockResourceAPIClient
	mockConnection        *MockConnection
	mockModelConfigClient *MockModelConfigAPIClient
	mockSharedClient      *MockSharedClient
	mockCharmhubClient    *MockCharmhubClient
}

func (s *ApplicationSuite) SetupTest() {}

func (s *ApplicationSuite) setupMocks(t *testing.T) *gomock.Controller {
	s.testModelName = "testmodel"

	ctlr := gomock.NewController(t)
	s.mockApplicationClient = NewMockApplicationAPIClient(ctlr)
	s.mockClient = NewMockClientAPIClient(ctlr)

	s.mockConnection = NewMockConnection(ctlr)
	s.mockConnection.EXPECT().Close().Return(nil).AnyTimes()

	s.mockResourceAPIClient = NewMockResourceAPIClient(ctlr)
	s.mockResourceAPIClient.EXPECT().ListResources(gomock.Any()).DoAndReturn(
		func(applications []string) ([]resources.ApplicationResources, error) {
			results := make([]resources.ApplicationResources, len(applications))
			return results, nil
		}).AnyTimes()

	s.mockModelConfigClient = NewMockModelConfigAPIClient(ctlr)
	minConfig := map[string]interface{}{
		"name":            "test",
		"type":            "manual",
		"uuid":            utils.MustNewUUID().String(),
		"controller-uuid": utils.MustNewUUID().String(),
		"firewall-mode":   "instance",
		"secret-backend":  "auto",
		"image-stream":    "testing",
	}
	cfg, err := config.New(true, minConfig)
	s.Require().NoError(err, "New config failed")
	attrs := cfg.AllAttrs()
	attrs["default-space"] = "alpha"
	s.mockModelConfigClient.EXPECT().ModelGet().Return(attrs, nil).AnyTimes()

	log := func(msg string, additionalFields ...map[string]interface{}) {
		s.T().Logf("logging from shared client %q, %+v", msg, additionalFields)
	}
	s.mockSharedClient = NewMockSharedClient(ctlr)
	s.mockSharedClient.EXPECT().Debugf(gomock.Any(), gomock.Any()).Do(log).AnyTimes()
	s.mockSharedClient.EXPECT().Errorf(gomock.Any(), gomock.Any()).Do(log).AnyTimes()
	s.mockSharedClient.EXPECT().Tracef(gomock.Any(), gomock.Any()).Do(log).AnyTimes()
	s.mockSharedClient.EXPECT().JujuLogger().Return(&jujuLoggerShim{}).AnyTimes()
	s.mockSharedClient.EXPECT().GetConnection(&s.testModelName).Return(s.mockConnection, nil).AnyTimes()

	s.mockCharmhubClient = NewMockCharmhubClient(ctlr)
	return ctlr
}

// TestAddPendingResourceCustomImageResourceProvidedCharmResourcesToAddExistsUploadPendingResourceCalled
// tests the case where charm has one image resources and one custom resource is provided.
// ResourceAPIClient.UploadPendingResource are is called but ResourceAPIClient.AddPendingResource is not called
// One resource ID is returned in the resource list.
func (s *ApplicationSuite) TestAddPendingResourceCustomImageResourceProvidedCharmResourcesToAddExistsUploadPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	deployValue := "ausf-image"
	path := "testrepo/udm/1:4"
	meta := charmresources.Meta{
		Name: deployValue,
		Type: charmresources.TypeContainerImage,
		Path: path,
	}
	ausfResourceID := "1111222"
	charmResourcesToAdd := make(map[string]charmresources.Meta)
	charmResourcesToAdd["ausf-image"] = meta
	resourcesToUse := make(map[string]string)
	resourcesToUse["ausf-image"] = "gatici/sdcore-ausf:1.4"
	revision := 433
	track := "1.5"
	url := "ch:amd64/jammy/sdcore-ausf-k8s-433"
	charmOrigin := apicharm.Origin{
		Source:       "charm-hub",
		ID:           "3V9Af7N3QcR4WdGiyF0fvZuJUSF7oMYe",
		Hash:         "e7b3ff9d328738861b701cd61ea7dd3670e74f5419c3f48c4ac67b10b307b888",
		Risk:         "edge",
		Revision:     &revision,
		Track:        &track,
		Architecture: "amd64",
		Base: corebase.Base{
			OS: "ubuntu",
			Channel: corebase.Channel{
				Track: "22.04",
				Risk:  "stable",
			},
		},
		InstanceKey: "_JsD_6xYr5kYP-gBz6wJ6lt6N1L-zslpIkXAUS-bu4w",
	}
	charmID := apiapplication.CharmID{
		URL:    url,
		Origin: charmOrigin,
	}

	aExp := s.mockResourceAPIClient.EXPECT()
	expectedResourceIDs := map[string]string{"ausf-image": ausfResourceID}

	aExp.UploadPendingResource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(ausfResourceID, nil)

	resourceIDs, err := addPendingResources(appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceCustomImageResourceProvidedNoCharmResourcesToAddEmptyResourceListReturned
// tests the case where charm do not have image resources and one custom resource is provided.
// ResourceAPIClient.AddPendingResource and ResourceAPIClient.UploadPendingResource are not called
// Empty resource list is returned.
func (s *ApplicationSuite) TestAddPendingResourceCustomImageResourceProvidedNoCharmResourcesToAddEmptyResourceListReturned() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	charmResourcesToAdd := make(map[string]charmresources.Meta)
	resourcesToUse := make(map[string]string)
	resourcesToUse["ausf-image"] = "gatici/sdcore-ausf:1.4"
	revision := 433
	track := "1.5"
	url := "ch:amd64/jammy/sdcore-ausf-k8s-433"
	charmOrigin := apicharm.Origin{
		Source:       "charm-hub",
		ID:           "3V9Af7N3QcR4WdGiyF0fvZuJUSF7oMYe",
		Hash:         "e7b3ff9d328738861b701cd61ea7dd3670e74f5419c3f48c4ac67b10b307b888",
		Risk:         "edge",
		Revision:     &revision,
		Track:        &track,
		Architecture: "amd64",
		Base: corebase.Base{
			OS: "ubuntu",
			Channel: corebase.Channel{
				Track: "22.04",
				Risk:  "stable",
			},
		},
		InstanceKey: "_JsD_6xYr5kYP-gBz6wJ6lt6N1L-zslpIkXAUS-bu4w",
	}

	charmID := apiapplication.CharmID{
		URL:    url,
		Origin: charmOrigin,
	}

	expectedResourceIDs := map[string]string{}
	resourceIDs, err := addPendingResources(appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceOneCustomResourceOneRevisionProvidedMultipleCharmResourcesToAddUploadPendingResourceAndAddPendingResourceCalled
// tests the case where charm has multiple image resources and one revision number and one custom resource is provided for different charm resources.
// ResourceAPIClient.AddPendingResource and ResourceAPIClient.UploadPendingResource is called.
func (s *ApplicationSuite) TestAddPendingResourceOneCustomResourceOneRevisionProvidedMultipleCharmResourcesToAddUploadPendingResourceAndAddPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	ausfDeployValue := "ausf-image"
	udmDeployValue := "udm-image"
	pathAusf := "testrepo/ausf/1:4"
	pathUdm := "testrepo/udm/1:4"
	metaAusf := charmresources.Meta{
		Name: ausfDeployValue,
		Type: charmresources.TypeContainerImage,
		Path: pathAusf,
	}
	metaUdm := charmresources.Meta{
		Name: udmDeployValue,
		Type: charmresources.TypeContainerImage,
		Path: pathUdm,
	}
	ausfResourceID := "1111222"
	udmResourceID := "1111444"
	charmResourcesToAdd := make(map[string]charmresources.Meta)
	charmResourcesToAdd["ausf-image"] = metaUdm
	charmResourcesToAdd["udm-image"] = metaAusf
	resourcesToUse := make(map[string]string)
	resourcesToUse["ausf-image"] = "gatici/sdcore-ausf:1.4"
	resourcesToUse["udm-image"] = "3"
	revision := 433
	track := "1.5"
	url := "ch:amd64/jammy/sdcore-ausf-k8s-433"
	charmOrigin := apicharm.Origin{
		Source:       "charm-hub",
		ID:           "3V9Af7N3QcR4WdGiyF0fvZuJUSF7oMYe",
		Hash:         "e7b3ff9d328738861b701cd61ea7dd3670e74f5419c3f48c4ac67b10b307b888",
		Risk:         "edge",
		Revision:     &revision,
		Track:        &track,
		Architecture: "amd64",
		Base: corebase.Base{
			OS: "ubuntu",
			Channel: corebase.Channel{
				Track: "22.04",
				Risk:  "stable",
			},
		},
		InstanceKey: "_JsD_6xYr5kYP-gBz6wJ6lt6N1L-zslpIkXAUS-bu4w",
	}
	charmID := apiapplication.CharmID{
		URL:    url,
		Origin: charmOrigin,
	}

	aExp := s.mockResourceAPIClient.EXPECT()
	expectedResourceIDs := map[string]string{"ausf-image": ausfResourceID, "udm-image": udmResourceID}

	aExp.UploadPendingResource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(ausfResourceID, nil)
	aExp.AddPendingResources(gomock.Any()).Return([]string{"1111444"}, nil)

	resourceIDs, err := addPendingResources(appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceOneRevisionProvidedMultipleCharmResourcesToAddOnlyAddPendingResourceCalled
// tests the case where charm has multiple image resources and revision number is provided for one resource.
// Only ResourceAPIClient.AddPendingResource called, ResourceAPIClient.UploadPendingResource is not called.
func (s *ApplicationSuite) TestAddPendingResourceOneRevisionProvidedMultipleCharmResourcesToAddOnlyAddPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	ausfDeployValue := "ausf-image"
	udmDeployValue := "udm-image"
	pathAusf := "testrepo/ausf/1:4"
	pathUdm := "testrepo/udm/1:4"
	metaAusf := charmresources.Meta{
		Name: ausfDeployValue,
		Type: charmresources.TypeContainerImage,
		Path: pathAusf,
	}
	metaUdm := charmresources.Meta{
		Name: udmDeployValue,
		Type: charmresources.TypeContainerImage,
		Path: pathUdm,
	}
	udmResourceID := "1111444"
	charmResourcesToAdd := make(map[string]charmresources.Meta)
	charmResourcesToAdd["ausf-image"] = metaUdm
	charmResourcesToAdd["udm-image"] = metaAusf
	resourcesToUse := make(map[string]string)
	resourcesToUse["udm-image"] = "3"
	revision := 433
	track := "1.5"
	url := "ch:amd64/jammy/sdcore-ausf-k8s-433"
	charmOrigin := apicharm.Origin{
		Source:       "charm-hub",
		ID:           "3V9Af7N3QcR4WdGiyF0fvZuJUSF7oMYe",
		Hash:         "e7b3ff9d328738861b701cd61ea7dd3670e74f5419c3f48c4ac67b10b307b888",
		Risk:         "edge",
		Revision:     &revision,
		Track:        &track,
		Architecture: "amd64",
		Base: corebase.Base{
			OS: "ubuntu",
			Channel: corebase.Channel{
				Track: "22.04",
				Risk:  "stable",
			},
		},
		InstanceKey: "_JsD_6xYr5kYP-gBz6wJ6lt6N1L-zslpIkXAUS-bu4w",
	}
	charmID := apiapplication.CharmID{
		URL:    url,
		Origin: charmOrigin,
	}

	aExp := s.mockResourceAPIClient.EXPECT()
	expectedResourceIDs := map[string]string{"udm-image": udmResourceID}
	aExp.AddPendingResources(gomock.Any()).Return([]string{udmResourceID}, nil)

	resourceIDs, err := addPendingResources(appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestUploadExistingPendingResourcesUploadSuccessful tests the case where ResourceAPIClient.Upload is successful.
// Error is not returned.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesUploadSuccessful() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: "myimage",
		Type:     "oci-image",
	}
	var pendingResources []apiapplication.PendingResourceUpload
	pendingResources = append(pendingResources, resource)
	fileSystem := osFilesystem{}
	aExp := s.mockResourceAPIClient.EXPECT()
	aExp.Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	err := uploadExistingPendingResources(appName, pendingResources, fileSystem, s.mockResourceAPIClient)
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestUploadExistingPendingResourcesUploadFailedReturnError tests the case where ResourceAPIClient.Upload failed.
// Returns error that upload failed for provided file name.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesUploadFailedReturnError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	fileName := "my-image"
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: fileName,
		Type:     "oci-image",
	}
	var pendingResources []apiapplication.PendingResourceUpload
	pendingResources = append(pendingResources, resource)
	fileSystem := osFilesystem{}
	aExp := s.mockResourceAPIClient.EXPECT()
	aExp.Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("upload failed for %s", fileName))

	err := uploadExistingPendingResources(appName, pendingResources, fileSystem, s.mockResourceAPIClient)
	s.Assert().Equal("upload failed for my-image", err.Error(), "Error is expected.")
}

// TestUploadExistingPendingResourcesResourceTypeUnknownReturnError tests the case where resource type is unknown.
// ResourceAPIClient.Upload is not called and returns error that resource type is invalid.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesResourceTypeUnknownReturnError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	var pendingResources []apiapplication.PendingResourceUpload
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: "my-image",
		Type:     "unknown",
	}
	pendingResources = append(pendingResources, resource)
	fileSystem := osFilesystem{}
	err := uploadExistingPendingResources(appName, pendingResources, fileSystem, s.mockResourceAPIClient)
	s.Assert().Equal("invalid type unknown for pending resource custom-image: unsupported resource type \"unknown\"", err.Error(), "Error is expected.")
}

// TestUploadExistingPendingResourcesInvalidFileNameReturnError tests the case where file path is not valid.
// ResourceAPIClient.Upload is not called and returns error that unable to open resource.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesInvalidFileNameReturnError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	var pendingResources []apiapplication.PendingResourceUpload
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: "",
		Type:     "oci-image",
	}
	pendingResources = append(pendingResources, resource)
	fileSystem := osFilesystem{}
	err := uploadExistingPendingResources(appName, pendingResources, fileSystem, s.mockResourceAPIClient)
	s.Assert().Equal("unable to open resource custom-image: filepath or registry path:  not valid", err.Error(), "Error is expected.")
}

func (s *ApplicationSuite) TestIsSubordinate() {
	defer s.setupMocks(s.T()).Finish()

	tests := []struct {
		metadataYAML string
		subordinate  bool
	}{{
		metadataYAML: `name: ntp
subordinate: true
maintainer: NTP Charm Maintainers <ntp-team@lists.launchpad.net>
summary: Network Time Protocol
description: |
  NTP, the Network Time Protocol, is used to keep computer clocks accurate
  by synchronizing them over the Internet or a local network, or by
  following an accurate hardware receiver that interprets GPS, DCF-77,
  NIST or similar time signals.
  .
  This charm can be deployed alongside principal charms to enable NTP
  management across deployed services.
tags:
  - misc
series:
  - focal
  - bionic
  - xenial
  - trusty
  - jammy
provides:
  ntpmaster:
    interface: ntp
requires:
  juju-info:
    interface: juju-info
    scope: container
  master:
    interface: ntp
peers:
  ntp-peers:
    interface: ntp`,
		subordinate: true,
	}, {
		metadataYAML: `name: postgresql
display-name: Charmed PostgreSQL VM
summary: Charmed PostgreSQL VM operator
description: |
    Charm to operate the PostgreSQL database on machines.
docs: https://discourse.charmhub.io/t/charmed-postgresql-documentation/9710
source: https://github.com/canonical/postgresql-operator
issues: https://github.com/canonical/postgresql-operator/issues
maintainers:
    - Canonical Data Platform <data-platform@lists.launchpad.net>
`,
		subordinate: false,
	}}

	for _, test := range tests {
		s.mockCharmhubClient.EXPECT().Info(gomock.Any(), gomock.Any(), gomock.Any()).Return(transport.InfoResponse{
			DefaultRelease: transport.InfoChannelMap{
				Revision: transport.InfoRevision{
					MetadataYAML: test.metadataYAML,
				},
			},
		}, nil)

		isSubordinate, err := isSubordinateCharm(context.Background(), s.mockCharmhubClient, "some-charm", "latest/stable")
		s.Assert().Equal(nil, err, "Error is not expected.")
		s.Assert().Equal(test.subordinate, isSubordinate, "expectedValue")
	}
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestApplicationSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSuite))
}
