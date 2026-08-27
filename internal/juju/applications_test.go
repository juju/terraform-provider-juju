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
	charmURL "github.com/juju/juju/domain/deployment/charm"
	charmresources "github.com/juju/juju/domain/deployment/charm/resource"
	"github.com/juju/juju/environs/config"
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

// fakeCharmsResolver is a hand-written fake implementing CharmsAPIResolver
// for unit tests, avoiding mockgen churn for a two-method interface.
type fakeCharmsResolver struct {
	resolveResult  []apicharms.ResolvedCharm
	resolveErr     error
	addCharmResult apicharm.Origin
	addCharmErr    error

	gotResolveURL *charmURL.URL
	gotAddOrigin  apicharm.Origin
}

func (f *fakeCharmsResolver) ResolveCharms(_ context.Context, charms []apicharms.CharmToResolve) ([]apicharms.ResolvedCharm, error) {
	if len(charms) == 1 {
		f.gotResolveURL = charms[0].URL
	}
	return f.resolveResult, f.resolveErr
}

func (f *fakeCharmsResolver) AddCharm(_ context.Context, curl *charmURL.URL, origin apicharm.Origin, _ bool) (apicharm.Origin, error) {
	f.gotAddOrigin = origin
	return f.addCharmResult, f.addCharmErr
}

// TestComputeCharmIDBaseChangeWithoutRevisionResolution tests that when
// base changes and revision is not pinned, resolution floats by channel+base
// and the base-compat guard does not block the update.
func (s *ApplicationSuite) TestComputeCharmIDBaseChangeWithoutRevisionResolution() {
	defer s.setupMocks(s.T()).Finish()
	appName := "traefik"

	oldRev := 377
	oldURL := charmURL.MustParseURL("ch:amd64/traefik-k8s-377")
	oldOrigin := apicharm.Origin{
		Source:       "charm-hub",
		Risk:         "stable",
		Revision:     &oldRev,
		Architecture: "amd64",
		Base: corebase.Base{
			OS:      "ubuntu",
			Channel: corebase.Channel{Track: "20.04", Risk: "stable"},
		},
	}

	s.mockApplicationClient.EXPECT().GetCharmURLOrigin(gomock.Any(), appName).Return(oldURL, oldOrigin, nil)

	newBase := corebase.Base{OS: "ubuntu", Channel: corebase.Channel{Track: "26.04", Risk: "stable"}}
	supportedBases := []corebase.Base{newBase}
	resolvedOrigin := oldOrigin
	resolvedOrigin.Base = newBase
	resolvedOrigin.Revision = nil
	resolvedURL := charmURL.MustParseURL("ch:amd64/traefik-k8s-378")

	fake := &fakeCharmsResolver{
		resolveResult: []apicharms.ResolvedCharm{
			{
				URL:            resolvedURL,
				Origin:         resolvedOrigin,
				SupportedBases: supportedBases,
			},
		},
		addCharmResult: resolvedOrigin,
	}

	client := s.getApplicationsClient()
	charmID, err := client.computeCharmID(s.T().Context(), &UpdateApplicationInput{
		AppName: appName,
		Channel: "latest/stable",
		Base:    "ubuntu@26.04",
	}, s.mockApplicationClient, fake)

	s.Assert().NoError(err)
	s.Assert().Equal("ch:amd64/traefik-k8s-378", charmID.URL)
	s.Assert().Equal(newBase, charmID.Origin.Base)
	// Resolve was asked with the revision dropped so the resolver can float
	// by channel+base rather than pinning the currently-deployed revision.
	s.Assert().Equal(-1, fake.gotResolveURL.Revision)
	// AddCharm received the origin with the resolved revision so the apiserver
	// is not silently re-resolved to the pre-update revision.
	s.Assert().Equal(resolvedOrigin.Revision, fake.gotAddOrigin.Revision)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestApplicationSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSuite))
}
