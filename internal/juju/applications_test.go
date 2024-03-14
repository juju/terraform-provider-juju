// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

// Basic imports
import (
	"context"
	"testing"

	"github.com/juju/juju/api"
	"github.com/juju/juju/api/base"
	"github.com/juju/juju/core/constraints"
	"github.com/juju/juju/core/model"
	"github.com/juju/juju/environs/config"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
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

	mockConnection        *MockConnection
	mockModelConfigClient *MockModelConfigAPIClient
	mockSharedClient      *MockSharedClient
}

func (s *ApplicationSuite) SetupTest() {}

func (s *ApplicationSuite) setupMocks(t *testing.T) *gomock.Controller {
	s.testModelName = "testmodel"

	ctlr := gomock.NewController(t)
	s.mockApplicationClient = NewMockApplicationAPIClient(ctlr)
	s.mockClient = NewMockClientAPIClient(ctlr)

	s.mockConnection = NewMockConnection(ctlr)
	s.mockConnection.EXPECT().Close().Return(nil).AnyTimes()

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
			ModelName: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
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
			ModelName: s.testModelName,
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
			ModelName: s.testModelName,
			AppName:   appName,
		})
	s.Require().NoError(err, "error from ReadApplicationWithRetryOnNotFound")
	s.Require().NotNil(resp, "ReadApplicationWithRetryOnNotFound response")

	s.Assert().Equal("testcharm", resp.Name)
	s.Assert().Equal("stable", resp.Channel)
	s.Assert().Equal(5, resp.Revision)
	s.Assert().Equal("ubuntu@22.04", resp.Base)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestApplicationSuite(t *testing.T) {
	suite.Run(t, new(ApplicationSuite))
}
