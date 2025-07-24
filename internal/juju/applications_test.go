// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

// Basic imports
import (
	"context"
	"fmt"
	"testing"

	charmresources "github.com/juju/charm/v12/resource"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	apiapplication "github.com/juju/juju/api/client/application"
	apicharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/charmhub/transport"
	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/core/resources"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v5"
	"github.com/juju/utils/v3"
	"github.com/juju/version/v2"
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

func (s *ApplicationSuite) getApplicationsClient() applicationsClient {
	return applicationsClient{
		SharedClient:      s.mockSharedClient,
		controllerVersion: version.Number{},
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

func (s *ApplicationSuite) TestReadApplicationRetry() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	aExp := s.mockApplicationClient.EXPECT()

	// First response is not found.
	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{{
		Error: &params.Error{Message: `application "testapplication" not found`, Code: "not found"},
	}}, nil)

	// Retry - expect ApplicationsInfo and Status to be called.
	// The second time return a real application.
	amdConst := constraints.MustParse("arch=amd64")
	infoResult := params.ApplicationInfoResult{
		Result: &params.ApplicationResult{
			Tag:         names.NewApplicationTag(appName).String(),
			Charm:       "ch:amd64/jammy/testcharm-5",
			Base:        params.Base{Name: "ubuntu", Channel: "22.04"},
			Channel:     "stable",
			Constraints: amdConst,
			Principal:   true,
		},
		Error: nil,
	}

	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{infoResult}, nil)
	getResult := &params.ApplicationGetResults{
		Application:       appName,
		CharmConfig:       nil,
		ApplicationConfig: nil,
		Charm:             "ch:amd64/jammy/testcharm-5",
		Base:              params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:           "stable",
		Constraints:       amdConst,
		EndpointBindings:  nil,
	}
	aExp.Get("master", appName).Return(getResult, nil)
	statusResult := &params.FullStatus{
		Applications: map[string]params.ApplicationStatus{appName: {
			Charm: "ch:amd64/jammy/testcharm-5",
			Units: map[string]params.UnitStatus{"testapplication/0": {
				Machine: "0",
			}},
		}},
	}
	s.mockClient.EXPECT().Status(gomock.Any()).Return(statusResult, nil)

	client := s.getApplicationsClient()
	resp, err := client.ReadApplicationWithRetryOnNotFound(context.Background(),
		&ReadApplicationInput{
			ModelUUID: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
}

func (s *ApplicationSuite) TestReadApplicationRetryDoNotPanic() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	aExp := s.mockApplicationClient.EXPECT()

	aExp.ApplicationsInfo(gomock.Any()).Return(nil, fmt.Errorf("don't panic"))

	client := s.getApplicationsClient()
	_, err := client.ReadApplicationWithRetryOnNotFound(context.Background(),
		&ReadApplicationInput{
			ModelUUID: s.testModelName,
			AppName:   appName,
		})
	s.Require().Error(err, "don't panic")
}

func (s *ApplicationSuite) TestReadApplicationRetryWaitForMachines() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	aExp := s.mockApplicationClient.EXPECT()

	// First response doesn't have enough machines.
	amdConst := constraints.MustParse("arch=amd64")
	infoResult := params.ApplicationInfoResult{
		Result: &params.ApplicationResult{
			Tag:         names.NewApplicationTag(appName).String(),
			Charm:       "ch:amd64/jammy/testcharm-5",
			Base:        params.Base{Name: "ubuntu", Channel: "22.04"},
			Channel:     "stable",
			Constraints: amdConst,
			Principal:   true,
		},
		Error: nil,
	}

	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{infoResult}, nil).Times(2)
	getResult := &params.ApplicationGetResults{
		Application:       appName,
		CharmConfig:       nil,
		ApplicationConfig: nil,
		Charm:             "ch:amd64/jammy/testcharm-5",
		Base:              params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:           "stable",
		Constraints:       amdConst,
		EndpointBindings:  nil,
	}
	aExp.Get("master", appName).Return(getResult, nil).Times(2)

	statusResult := &params.FullStatus{
		Applications: map[string]params.ApplicationStatus{appName: {
			Charm: "ch:amd64/jammy/testcharm-5",
			Units: map[string]params.UnitStatus{
				"testapplication/0": {
					Machine: "0",
				},
				"testapplication/1": {}},
		}},
	}
	s.mockClient.EXPECT().Status(gomock.Any()).Return(statusResult, nil)

	statusResult2 := &params.FullStatus{
		Applications: map[string]params.ApplicationStatus{appName: {
			Charm: "ch:amd64/jammy/testcharm-5",
			Units: map[string]params.UnitStatus{
				"testapplication/0": {
					Machine: "0",
				},
				"testapplication/1": {
					Machine: "1",
				}},
		}},
	}
	s.mockClient.EXPECT().Status(gomock.Any()).Return(statusResult2, nil)

	client := s.getApplicationsClient()
	resp, err := client.ReadApplicationWithRetryOnNotFound(context.Background(),
		&ReadApplicationInput{
			ModelUUID: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
	s.Assert().Equal("0,1", resp.Placement)
}

func (s *ApplicationSuite) TestReadApplicationRetrySubordinate() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	aExp := s.mockApplicationClient.EXPECT()

	amdConst := constraints.MustParse("arch=amd64")
	infoResult := params.ApplicationInfoResult{
		Result: &params.ApplicationResult{
			Tag:         names.NewApplicationTag(appName).String(),
			Charm:       "ch:amd64/jammy/testcharm-5",
			Base:        params.Base{Name: "ubuntu", Channel: "22.04"},
			Channel:     "stable",
			Constraints: amdConst,
			Principal:   false,
		},
		Error: nil,
	}

	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{infoResult}, nil)
	getResult := &params.ApplicationGetResults{
		Application:       appName,
		CharmConfig:       nil,
		ApplicationConfig: nil,
		Charm:             "ch:amd64/jammy/testcharm-5",
		Base:              params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:           "stable",
		Constraints:       amdConst,
		EndpointBindings:  nil,
	}
	aExp.Get("master", appName).Return(getResult, nil)
	statusResult := &params.FullStatus{
		Applications: map[string]params.ApplicationStatus{appName: {
			Charm: "ch:amd64/jammy/testcharm-5",
		}},
	}
	s.mockClient.EXPECT().Status(gomock.Any()).Return(statusResult, nil)

	client := s.getApplicationsClient()
	resp, err := client.ReadApplicationWithRetryOnNotFound(context.Background(),
		&ReadApplicationInput{
			ModelUUID: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
}

// TestReadApplicationRetryNotFoundStorageNotFoundError tests the case where the first response is a storage not found error.
// The second response is a real application.
func (s *ApplicationSuite) TestReadApplicationRetryNotFoundStorageNotFoundError() {
	defer s.setupMocks(s.T()).Finish()
	s.mockSharedClient.EXPECT().ModelType(gomock.Any()).Return(model.IAAS, nil).AnyTimes()

	appName := "testapplication"
	aExp := s.mockApplicationClient.EXPECT()

	// First response is a storage not found error.
	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{{
		Error: &params.Error{Message: `storage "testapplication" not found`, Code: "not found"},
	}}, nil)

	// Retry - expect ApplicationsInfo and Status to be called.
	// The second time return a real application.
	amdConst := constraints.MustParse("arch=amd64")
	infoResult := params.ApplicationInfoResult{
		Result: &params.ApplicationResult{
			Tag:         names.NewApplicationTag(appName).String(),
			Charm:       "ch:amd64/jammy/testcharm-5",
			Base:        params.Base{Name: "ubuntu", Channel: "22.04"},
			Channel:     "stable",
			Constraints: amdConst,
			Principal:   true,
		},
		Error: nil,
	}

	aExp.ApplicationsInfo(gomock.Any()).Return([]params.ApplicationInfoResult{infoResult}, nil)
	getResult := &params.ApplicationGetResults{
		Application:       appName,
		CharmConfig:       nil,
		ApplicationConfig: nil,
		Charm:             "ch:amd64/jammy/testcharm-5",
		Base:              params.Base{Name: "ubuntu", Channel: "22.04"},
		Channel:           "stable",
		Constraints:       amdConst,
		EndpointBindings:  nil,
	}
	aExp.Get("master", appName).Return(getResult, nil)
	statusResult := &params.FullStatus{
		Applications: map[string]params.ApplicationStatus{appName: {
			Charm: "ch:amd64/jammy/testcharm-5",
			Units: map[string]params.UnitStatus{"testapplication/0": {
				Machine: "0",
			}},
		}},
	}
	s.mockClient.EXPECT().Status(gomock.Any()).Return(statusResult, nil)

	client := s.getApplicationsClient()
	resp, err := client.ReadApplicationWithRetryOnNotFound(context.Background(),
		&ReadApplicationInput{
			ModelUUID: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
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
