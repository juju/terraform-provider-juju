// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type JujuSuite struct {
	suite.Suite

	testModelName *string

	mockConnection   *MockConnection
	mockSharedClient *MockSharedClient
}

func (s *JujuSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := gomock.NewController(t)

	s.mockConnection = NewMockConnection(ctlr)
	s.mockConnection.EXPECT().Close().Return(nil).AnyTimes()

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
	s.mockSharedClient.EXPECT().GetConnection(s.testModelName).Return(s.mockConnection, nil).AnyTimes()

	return ctlr
}
