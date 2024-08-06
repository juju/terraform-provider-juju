package juju

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type JaasSuite struct {
	suite.Suite
	JujuSuite

	testModelName string

	mockJaasClient *MockSecretAPIClient
}

func (s *JaasSuite) SetupTest() {}

func (s *JaasSuite) setupMocks(t *testing.T) *gomock.Controller {
	s.testModelName = "test-secret-model"

	ctlr := s.JujuSuite.setupMocks(t)
	s.mockJaasClient = NewMockSecretAPIClient(ctlr)

	return ctlr
}
