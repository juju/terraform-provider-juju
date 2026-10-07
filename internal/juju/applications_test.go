// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

// Basic imports
import (
	"context"
	"fmt"
	"io"
	"testing"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharms "github.com/juju/juju/api/client/charms"
	apiresources "github.com/juju/juju/api/client/resources"
	apicharm "github.com/juju/juju/api/common/charm"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resource"
	"github.com/juju/juju/domain/deployment/charm"
	charmresources "github.com/juju/juju/domain/deployment/charm/resource"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/utils/v4"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type ApplicationSuite struct {
	suite.Suite

	testModelUUID string

	mockApplicationClient *MockApplicationAPIClient
	mockClient            *MockClientAPIClient
	mockResourceAPIClient *MockResourceAPIClient
	mockConnection        *MockConnection
	mockModelConfigClient *MockModelConfigAPIClient
	mockSharedClient      *MockSharedClient
}

func (s *ApplicationSuite) SetupTest() {}

func (s *ApplicationSuite) setupMocks(t *testing.T) *gomock.Controller {
	s.testModelUUID = "test-uuid"

	ctlr := gomock.NewController(t)
	s.mockApplicationClient = NewMockApplicationAPIClient(ctlr)
	s.mockClient = NewMockClientAPIClient(ctlr)

	s.mockConnection = NewMockConnection(ctlr)
	s.mockConnection.EXPECT().Close().Return(nil).AnyTimes()

	s.mockResourceAPIClient = NewMockResourceAPIClient(ctlr)
	s.mockResourceAPIClient.EXPECT().ListResources(gomock.Any(), gomock.Any()).DoAndReturn(
		func(ctx context.Context, applications []string) ([]resource.ApplicationResources, error) {
			results := make([]resource.ApplicationResources, len(applications))
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
	s.mockModelConfigClient.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).AnyTimes()

	log := func(msg string, additionalFields ...map[string]interface{}) {
		s.T().Logf("logging from shared client %q, %+v", msg, additionalFields)
	}
	logErr := func(err error, msg string) {
		s.T().Logf("error logging from shared client %q: %v", msg, err)
	}
	s.mockSharedClient = NewMockSharedClient(ctlr)
	s.mockSharedClient.EXPECT().Debugf(gomock.Any(), gomock.Any()).Do(log).AnyTimes()
	s.mockSharedClient.EXPECT().Errorf(gomock.Any(), gomock.Any()).Do(logErr).AnyTimes()
	s.mockSharedClient.EXPECT().Tracef(gomock.Any(), gomock.Any()).Do(log).AnyTimes()
	s.mockSharedClient.EXPECT().JujuLogger().Return(&jujuLoggerShim{}).AnyTimes()
	s.mockSharedClient.EXPECT().GetConnection(gomock.Any(), &s.testModelUUID).Return(s.mockConnection, nil).AnyTimes()

	return ctlr
}

func (s *ApplicationSuite) getApplicationsClient() applicationsClient {
	return applicationsClient{
		SharedClient: s.mockSharedClient,
		getApplicationAPIClient: func(_ base.APICallCloser) ApplicationAPIClient {
			return s.mockApplicationClient
		},
		getClientAPIClient: func(_ api.Connection) ClientAPIClient {
			return s.mockClient
		},
		getModelConfigAPIClient: func(_ api.Connection) ModelConfigAPIClient {
			return s.mockModelConfigClient
		},
		getResourceAPIClient: func(_ api.Connection) (ResourceAPIClient, error) {
			return s.mockResourceAPIClient, nil
		},
	}
}

// TestAddPendingResourceCustomImageResourceProvidedCharmResourcesToAddExistsUploadPendingResourceCalled
// tests the case where charm has one image resources and one custom resource is provided.
// ResourceAPIClient.UploadPendingResource are is called but ResourceAPIClient.AddPendingResource is not called
// One resource ID is returned in the resource list.
func (s *ApplicationSuite) TestAddPendingResourceCustomImageResourceProvidedCharmResourcesToAddExistsUploadPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()

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
	resourcesToUse := make(map[string]CharmResource)
	resourcesToUse["ausf-image"] = CharmResource{OCIImageURL: "gatici/sdcore-ausf:1.4"}
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

	aExp.UploadPendingResource(gomock.Any(), gomock.Any()).Return(ausfResourceID, nil)

	resourceIDs, err := addPendingResources(s.T().Context(), appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceCustomImageResourceProvidedNoCharmResourcesToAddEmptyResourceListReturned
// tests the case where charm do not have image resources and one custom resource is provided.
// ResourceAPIClient.AddPendingResource and ResourceAPIClient.UploadPendingResource are not called
// Empty resource list is returned.
func (s *ApplicationSuite) TestAddPendingResourceCustomImageResourceProvidedNoCharmResourcesToAddEmptyResourceListReturned() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	charmResourcesToAdd := make(map[string]charmresources.Meta)
	resourcesToUse := make(map[string]CharmResource)
	resourcesToUse["ausf-image"] = CharmResource{OCIImageURL: "gatici/sdcore-ausf:1.4"}
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
	resourceIDs, err := addPendingResources(s.T().Context(), appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceOneCustomResourceOneRevisionProvidedMultipleCharmResourcesToAddUploadPendingResourceAndAddPendingResourceCalled
// tests the case where charm has multiple image resources and one revision number and one custom resource is provided for different charm resources.
// ResourceAPIClient.AddPendingResource and ResourceAPIClient.UploadPendingResource is called.
func (s *ApplicationSuite) TestAddPendingResourceOneCustomResourceOneRevisionProvidedMultipleCharmResourcesToAddUploadPendingResourceAndAddPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()

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
	resourcesToUse := make(map[string]CharmResource)
	resourcesToUse["ausf-image"] = CharmResource{OCIImageURL: "gatici/sdcore-ausf:1.4"}
	resourcesToUse["udm-image"] = CharmResource{RevisionNumber: "3"}
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

	aExp.UploadPendingResource(gomock.Any(), gomock.Any()).Return(ausfResourceID, nil)
	aExp.AddPendingResources(gomock.Any(), gomock.Any()).Return([]string{"1111444"}, nil)

	resourceIDs, err := addPendingResources(s.T().Context(), appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestAddPendingResourceOneRevisionProvidedMultipleCharmResourcesToAddOnlyAddPendingResourceCalled
// tests the case where charm has multiple image resources and revision number is provided for one resource.
// Only ResourceAPIClient.AddPendingResource called, ResourceAPIClient.UploadPendingResource is not called.
func (s *ApplicationSuite) TestAddPendingResourceOneRevisionProvidedMultipleCharmResourcesToAddOnlyAddPendingResourceCalled() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()

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
	charmResourcesToAdd["ausf-image"] = metaAusf
	charmResourcesToAdd["udm-image"] = metaUdm
	resourcesToUse := make(map[string]CharmResource)
	resourcesToUse["udm-image"] = CharmResource{RevisionNumber: "3"}
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
	expectedResourceIDs := map[string]string{
		"udm-image":  udmResourceID,
		"ausf-image": ausfResourceID,
	}
	aExp.AddPendingResources(s.T().Context(), apiresources.AddPendingResourcesArgs{
		ApplicationID: appName,
		CharmID: apiresources.CharmID{
			URL:    charmID.URL,
			Origin: charmID.Origin,
		},
		Resources: []charmresources.Resource{
			{
				Meta:     metaAusf,
				Origin:   charmresources.OriginStore,
				Revision: -1,
			},
			{
				Meta:     metaUdm,
				Origin:   charmresources.OriginStore,
				Revision: 3,
			},
		},
	}).Return([]string{ausfResourceID, udmResourceID}, nil)

	resourceIDs, err := addPendingResources(s.T().Context(), appName, charmResourcesToAdd, resourcesToUse, charmID, s.mockResourceAPIClient)
	s.Assert().Equal(resourceIDs, expectedResourceIDs, "Resource IDs does not match.")
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestUploadExistingPendingResourcesUploadSuccessful tests the case where ResourceAPIClient.Upload is successful.
// Error is not returned.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesUploadSuccessful() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: "myimage",
		Type:     "oci-image",
	}
	pendingResources := []apiapplication.PendingResourceUpload{resource}
	charmResources := map[string]CharmResource{
		"custom-image": {
			OCIImageURL:      "some-url",
			RegistryUser:     "username",
			RegistryPassword: "password",
		},
	}
	aExp := s.mockResourceAPIClient.EXPECT()
	aExp.Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	err := uploadExistingPendingResources(s.T().Context(), appName, pendingResources, charmResources, s.mockResourceAPIClient)
	s.Assert().Equal(nil, err, "Error is not expected.")
}

// TestUploadExistingPendingResourcesUploadFailedReturnError tests the case where ResourceAPIClient.Upload failed.
// Returns error that upload failed for provided file name.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesUploadFailedReturnError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	fileName := "my-image"
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: fileName,
		Type:     "oci-image",
	}
	pendingResources := []apiapplication.PendingResourceUpload{resource}
	charmResources := map[string]CharmResource{
		"custom-image": {
			OCIImageURL:      "some-url",
			RegistryUser:     "username",
			RegistryPassword: "password",
		},
	}
	aExp := s.mockResourceAPIClient.EXPECT()
	aExp.Upload(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(fmt.Errorf("upload failed for %s", fileName))

	err := uploadExistingPendingResources(s.T().Context(), appName, pendingResources, charmResources, s.mockResourceAPIClient)
	s.Assert().Equal("upload failed for my-image", err.Error(), "Error is expected.")
}

// TestUploadExistingPendingResourcesResourceTypeUnknownReturnError tests the case where resource type is unknown.
// ResourceAPIClient.Upload is not called and returns error that resource type is invalid.
func (s *ApplicationSuite) TestUploadExistingPendingResourcesResourceTypeUnknownReturnError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resource := apiapplication.PendingResourceUpload{
		Name:     "custom-image",
		Filename: "my-image",
		Type:     "unknown",
	}
	pendingResources := []apiapplication.PendingResourceUpload{resource}
	charmResources := map[string]CharmResource{
		"custom-image": {
			OCIImageURL:      "some-url",
			RegistryUser:     "username",
			RegistryPassword: "password",
		},
	}
	err := uploadExistingPendingResources(s.T().Context(), appName, pendingResources, charmResources, s.mockResourceAPIClient)
	s.Assert().Equal("invalid type unknown for pending resource custom-image: unsupported resource type \"unknown\"", err.Error(), "Error is expected.")
}

func (s *ApplicationSuite) TestApplicationUploadOCIResource() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resourceName := "myResource"
	client := s.getApplicationsClient()

	charmResource := CharmResource{
		OCIImageURL:      "some-url",
		RegistryUser:     "username",
		RegistryPassword: "password",
	}
	resourceContent, err := charmResource.MarhsalYaml()
	s.Assert().NoError(err)

	s.mockApplicationClient.EXPECT().DeployFromRepository(gomock.Any(), gomock.Any()).Return(
		apiapplication.DeployInfo{Name: appName},
		[]apiapplication.PendingResourceUpload{
			{
				Name:     resourceName,
				Filename: "arbitrary-path",
				Type:     "oci-image",
			},
		}, nil)

	s.mockResourceAPIClient.EXPECT().Upload(gomock.Any(), appName, resourceName, "arbitrary-path", "", gomock.Any()).
		DoAndReturn(func(ctx context.Context, s1, s2, s3, s4 string, rs io.ReadSeeker) error {
			uploadedContent, err := io.ReadAll(rs)
			s.Assert().NoError(err)
			s.Assert().Equal(resourceContent, uploadedContent)
			return nil
		})

	err = client.deployFromRepository(s.T().Context(), s.mockApplicationClient, s.mockResourceAPIClient, transformedCreateApplicationInput{
		applicationName: appName,
		resources:       map[string]CharmResource{"myResource": charmResource},
	})
	s.Assert().NoError(err)
}

func (s *ApplicationSuite) TestApplicationForbidFileUpload() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resourceName := "myResource"
	client := s.getApplicationsClient()

	s.mockApplicationClient.EXPECT().DeployFromRepository(gomock.Any(), gomock.Any()).Return(
		apiapplication.DeployInfo{Name: appName},
		[]apiapplication.PendingResourceUpload{
			{
				Name:     resourceName,
				Filename: "arbitrary-path",
				Type:     "file",
			},
		}, nil)

	err := client.deployFromRepository(s.T().Context(), s.mockApplicationClient, s.mockResourceAPIClient, transformedCreateApplicationInput{
		applicationName: appName,
		resources:       map[string]CharmResource{"myResource": {}},
	})
	s.Assert().ErrorContains(err, "uploading local resource of type file for resource myResource not supported")
}

func (s *ApplicationSuite) TestApplicationDeployWithRevision() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	client := s.getApplicationsClient()

	charmResource := CharmResource{
		RevisionNumber: "10",
	}

	s.mockApplicationClient.EXPECT().DeployFromRepository(gomock.Any(), gomock.Any()).Return(
		apiapplication.DeployInfo{Name: appName}, nil, nil)

	err := client.deployFromRepository(s.T().Context(), s.mockApplicationClient, s.mockResourceAPIClient, transformedCreateApplicationInput{
		applicationName: appName,
		resources:       map[string]CharmResource{"myResource": charmResource},
	})
	s.Assert().NoError(err)
}

// TestPartialApplicationDeployError tests the case where deployFromRepository returns pending resources to upload
// and uploadExistingPendingResources fails. We verify that the error can be unwrapped to an ApplicationPartiallyCreatedError
// indicating to the caller that the application was partially created.
func (s *ApplicationSuite) TestPartialApplicationDeployError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any(), gomock.Any()).Return(model.IAAS, nil).AnyTimes()
	appName := "testapplication"
	resourceName := "myResource"
	client := s.getApplicationsClient()

	s.mockApplicationClient.EXPECT().DeployFromRepository(gomock.Any(), gomock.Any()).Return(
		apiapplication.DeployInfo{Name: appName},
		[]apiapplication.PendingResourceUpload{
			{
				Name:     resourceName,
				Filename: "arbitrary-path",
				Type:     "oci-image",
			},
		}, nil)

	s.mockResourceAPIClient.EXPECT().Upload(gomock.Any(), appName, resourceName, "arbitrary-path", "", gomock.Any()).
		Return(fmt.Errorf("upload failed"))

	err := client.deployFromRepository(s.T().Context(), s.mockApplicationClient, s.mockResourceAPIClient, transformedCreateApplicationInput{
		applicationName: appName,
		resources: map[string]CharmResource{"myResource": {
			OCIImageURL: "some-url",
		}},
	})
	s.Assert().ErrorAs(err, &ApplicationPartiallyCreatedError{})
}

func (s *ApplicationSuite) TestComputeCharmIDChannel() {
	revision := 94
	tests := []struct {
		name            string
		oldChannel      string
		newChannel      string
		revision        *int
		expectedChannel string
	}{
		{name: "change track", oldChannel: "latest/edge", newChannel: "2.0/edge", expectedChannel: "2.0/edge"},
		{name: "change risk", oldChannel: "latest/edge", newChannel: "latest/stable", expectedChannel: "latest/stable"},
		{name: "remove branch", oldChannel: "latest/edge/testing", newChannel: "latest/edge", expectedChannel: "latest/edge"},
		{name: "remove branch with revision", oldChannel: "latest/edge/testing", newChannel: "latest/edge", revision: &revision, expectedChannel: "latest/edge"},
		{name: "remove branch and channel", oldChannel: "2.0/edge/testing", newChannel: "stable", expectedChannel: "stable"},
		{name: "change branch", oldChannel: "latest/edge/testing", newChannel: "latest/edge/other", expectedChannel: "latest/edge/other"},
		{name: "add branch", oldChannel: "latest/edge", newChannel: "latest/edge/testing", expectedChannel: "latest/edge/testing"},
		{name: "change full channel", oldChannel: "latest/edge/testing", newChannel: "2.0/beta/other", expectedChannel: "2.0/beta/other"},
		{name: "revision only preserves channel", oldChannel: "2.0/beta/testing", revision: &revision, expectedChannel: "2.0/beta/testing"},
	}
	for _, test := range tests {
		s.Run(test.name, func() {
			s.setupMocks(s.T())

			client := s.getApplicationsClient()
			oldChannel, err := charm.ParseChannel(test.oldChannel)
			s.Require().NoError(err)

			assertChannel := func(origin params.CharmOrigin) {
				channel := charm.Channel{Risk: charm.Risk(origin.Risk)}
				if origin.Track != nil {
					channel.Track = *origin.Track
				}
				if origin.Branch != nil {
					channel.Branch = *origin.Branch
				}
				s.Assert().Equal(test.expectedChannel, channel.String())
			}

			oldURL, err := charm.ParseURL("ch:amd64/test-charm-93")
			s.Require().NoError(err)
			oldRevision := 93
			oldOrigin := apicharm.Origin{
				Source:       "charm-hub",
				Type:         "charm",
				Track:        new(oldChannel.Track),
				Risk:         string(oldChannel.Risk),
				Revision:     &oldRevision,
				Architecture: "amd64",
				Base: corebase.Base{
					OS:      "ubuntu",
					Channel: corebase.Channel{Track: "22.04", Risk: "stable"},
				},
			}
			if oldChannel.Branch != "" {
				oldOrigin.Branch = new(oldChannel.Branch)
			}
			s.mockApplicationClient.EXPECT().GetCharmURLOrigin(gomock.Any(), "testapplication").Return(oldURL, oldOrigin, nil)
			s.mockConnection.EXPECT().BestFacadeVersion("Charms").Return(7)
			s.mockConnection.EXPECT().APICall(gomock.Any(), "Charms", 7, "", "ResolveCharms", gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, objType string, version int, id, request string, args, response any) error {
					resolveArgs := args.(params.ResolveCharmsWithChannel)
					s.Require().Len(resolveArgs.Resolve, 1)
					requested := resolveArgs.Resolve[0]
					assertChannel(requested.Origin)
					if test.revision != nil {
						s.Assert().Equal(test.revision, requested.Origin.Revision)
					}
					response.(*params.ResolveCharmWithChannelResults).Results = []params.ResolveCharmWithChannelResult{{
						URL:            requested.Reference,
						Origin:         requested.Origin,
						SupportedBases: []params.Base{{Name: "ubuntu", Channel: "22.04/stable"}},
					}}
					return nil
				})
			s.mockConnection.EXPECT().APICall(gomock.Any(), "Charms", 7, "", "AddCharm", gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, objType string, version int, id, request string, args, response any) error {
					addArgs := args.(params.AddCharmWithOrigin)
					assertChannel(addArgs.Origin)
					response.(*params.CharmOriginResult).Origin = addArgs.Origin
					return nil
				})

			charmID, err := client.computeCharmID(s.T().Context(), &UpdateApplicationInput{
				AppName:  "testapplication",
				Channel:  test.newChannel,
				Revision: test.revision,
			}, s.mockApplicationClient, apicharms.NewClient(s.mockConnection))
			s.Require().NoError(err)
			assertChannel(charmID.Origin.ParamsCharmOrigin())
		})
	}
}

func (s *ApplicationSuite) TestReadApplicationPreservesDeadApplicationError() {
	defer s.setupMocks(s.T()).Finish()
	client := s.getApplicationsClient()

	s.mockApplicationClient.EXPECT().ApplicationsInfo(gomock.Any(), gomock.Any()).Return(
		[]params.ApplicationInfoResult{{
			Error: &params.Error{
				Message: "querying application constraints for application \"testapplication\": application is dead",
			},
		}}, nil)

	_, err := client.ReadApplication(s.T().Context(), &ReadApplicationInput{
		ModelUUID: s.testModelUUID,
		AppName:   "testapplication",
	})
	s.Assert().ErrorContains(err, "application is dead")
	s.Assert().NotErrorIs(err, ApplicationNotFoundError)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestApplicationSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSuite))
}
