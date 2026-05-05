// Copyright 2024 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"encoding/base64"
	"errors"
	"testing"

	"github.com/juju/juju/api"
	apisecrets "github.com/juju/juju/api/client/secrets"
	coresecrets "github.com/juju/juju/core/secrets"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type SecretSuite struct {
	suite.Suite
	JujuSuite

	mockSecretClient *MockSecretAPIClient
}

func (s *SecretSuite) SetupSuite() {
	s.testModelName = strPtr("test-secret-model")
}

func (s *SecretSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := s.JujuSuite.setupMocks(t)
	s.mockSecretClient = NewMockSecretAPIClient(ctlr)

	return ctlr
}

func (s *SecretSuite) getSecretsClient() secretsClient {
	return secretsClient{
		SharedClient: s.mockSharedClient,
		getSecretAPIClient: func(connection api.Connection) SecretAPIClient {
			return s.mockSecretClient
		},
	}
}

func (s *SecretSuite) TestCreateSecret() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	decodedValue := map[string]string{"key": "value"}
	encodedValue := map[string]string{"key": base64.StdEncoding.EncodeToString([]byte("value"))}

	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)
	s.mockSecretClient.EXPECT().CreateSecret(gomock.Any(),
		"test-secret", "test info", encodedValue,
	).Return(secretURI.ID, nil).AnyTimes()

	client := s.getSecretsClient()
	output, err := client.CreateSecret(s.T().Context(), &CreateSecretInput{
		ModelUUID: *s.testModelName,
		Name:      "test-secret",
		Value:     decodedValue,
		Info:      "test info",
	})
	s.Require().NoError(err)

	s.Assert().NotNil(output)
	s.Assert().Equal(secretURI.ID, output.SecretId)
}

func (s *SecretSuite) TestCreateSecretError() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	errBoom := errors.New("boom")

	decodedValue := map[string]string{"key": "value"}
	encodedValue := map[string]string{"key": base64.StdEncoding.EncodeToString([]byte("value"))}

	s.mockSecretClient.EXPECT().CreateSecret(
		gomock.Any(),
		"test-secret", "test info", encodedValue,
	).Return("", errBoom).AnyTimes()

	client := s.getSecretsClient()
	output, err := client.CreateSecret(s.T().Context(), &CreateSecretInput{
		ModelUUID: *s.testModelName,
		Name:      "test-secret",
		Value:     decodedValue,
		Info:      "test info",
	})
	s.Require().Error(err)

	s.Assert().Equal(output, CreateSecretOutput{})
	s.Assert().Equal(errBoom, err)
}

func (s *SecretSuite) TestReadSecret() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)
	secretName := "test-secret"
	secretRevision := 1

	value := base64.StdEncoding.EncodeToString([]byte("value"))
	s.mockSecretClient.EXPECT().ListSecrets(gomock.Any(),
		true, coresecrets.Filter{
			URI:      secretURI,
			Label:    &secretName,
			Revision: &secretRevision,
		},
	).Return([]apisecrets.SecretDetails{
		{
			Metadata: coresecrets.SecretMetadata{
				URI:     secretURI,
				Version: 1,
			},
			Revisions: []coresecrets.SecretRevisionMetadata{
				{
					Revision: 1,
				},
			},
			Value: coresecrets.NewSecretValue(map[string]string{"key": value}),
			Error: "",
		},
	}, nil).AnyTimes()

	client := s.getSecretsClient()
	output, err := client.ReadSecret(s.T().Context(), &ReadSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
		Name:      &secretName,
		Revision:  &secretRevision,
	})
	s.Require().NoError(err)

	s.Assert().NotNil(output)
	s.Assert().Equal("value", output.Value["key"])
}

func (s *SecretSuite) TestReadSecretError() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)

	errBoom := errors.New("boom")
	s.mockSecretClient.EXPECT().ListSecrets(
		gomock.Any(),
		true, coresecrets.Filter{
			URI: secretURI,
		},
	).Return([]apisecrets.SecretDetails{
		{
			Error: errBoom.Error(),
		},
	}, nil).AnyTimes()

	client := s.getSecretsClient()
	output, err := client.ReadSecret(s.T().Context(), &ReadSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
	})
	s.Require().Error(err)

	s.Assert().Equal(output, ReadSecretOutput{})
	s.Assert().Equal(errBoom, err)
}

func (s *SecretSuite) TestUpdateSecretWithRenaming() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	newSecretName := "test-secret2"
	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	secretInfo := "secret info"
	autoPrune := true

	decodedValue := map[string]string{"key": "value"}
	encodedValue := map[string]string{"key": base64.StdEncoding.EncodeToString([]byte("value"))}

	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().UpdateSecret(
		gomock.Any(),
		secretURI, "", &autoPrune, newSecretName, "secret info", encodedValue,
	).Return(nil).AnyTimes()

	client := s.getSecretsClient()
	err = client.UpdateSecret(s.T().Context(), &UpdateSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
		Name:      &newSecretName,
		Value:     &decodedValue,
		AutoPrune: &autoPrune,
		Info:      &secretInfo,
	})
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().ListSecrets(
		gomock.Any(),
		true, coresecrets.Filter{URI: secretURI},
	).Return([]apisecrets.SecretDetails{
		{
			Metadata: coresecrets.SecretMetadata{
				URI:     secretURI,
				Version: 1,
			},
			Revisions: []coresecrets.SecretRevisionMetadata{
				{
					Revision: 1,
				},
			},
			Value: coresecrets.NewSecretValue(encodedValue),
			Error: "",
		},
	}, nil).Times(1)

	// read secret and check if value is updated
	output, err := client.ReadSecret(s.T().Context(), &ReadSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
	})
	s.Require().NoError(err)

	s.Assert().NotNil(output)
}

func (s *SecretSuite) TestUpdateSecret() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	secretInfo := "secret info"
	autoPrune := true

	decodedValue := map[string]string{"key": "value"}
	encodedValue := map[string]string{"key": base64.StdEncoding.EncodeToString([]byte("value"))}

	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().UpdateSecret(
		gomock.Any(),
		secretURI, "", &autoPrune, "", secretInfo, encodedValue,
	).Return(nil).AnyTimes()

	client := s.getSecretsClient()
	err = client.UpdateSecret(s.T().Context(), &UpdateSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
		Value:     &decodedValue,
		AutoPrune: &autoPrune,
		Info:      &secretInfo,
	})
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().ListSecrets(
		gomock.Any(),
		true, coresecrets.Filter{URI: secretURI},
	).Return([]apisecrets.SecretDetails{
		{
			Metadata: coresecrets.SecretMetadata{
				URI:         secretURI,
				Version:     1,
				Description: secretInfo,
			},
			Revisions: []coresecrets.SecretRevisionMetadata{
				{
					Revision: 1,
				},
			},
			Value: coresecrets.NewSecretValue(encodedValue),
			Error: "",
		},
	}, nil).Times(1)

	// read secret and check if secret info is updated
	output, err := client.ReadSecret(s.T().Context(), &ReadSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
	})
	s.Require().NoError(err)

	s.Assert().NotNil(output)
}

func (s *SecretSuite) TestDeleteSecret() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	secretId := "secret:9m4e2mr0ui3e8a215n4g"

	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().RemoveSecret(gomock.Any(), secretURI, "", nil).Return(nil).AnyTimes()

	client := s.getSecretsClient()
	err = client.DeleteSecret(s.T().Context(), &DeleteSecretInput{
		SecretId:  secretId,
		ModelUUID: *s.testModelName,
	})
	s.Assert().NoError(err)
}

func (s *SecretSuite) TestUpdateAccessSecret() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	secretId := "secret:9m4e2mr0ui3e8a215n4g"
	applications := []string{"app1", "app2"}

	secretURI, err := coresecrets.ParseURI(secretId)
	s.Require().NoError(err)

	s.mockSecretClient.EXPECT().GrantSecret(gomock.Any(), secretURI, "", applications).Return([]error{nil}, nil).AnyTimes()
	s.mockSecretClient.EXPECT().RevokeSecret(gomock.Any(), secretURI, "", applications).Return([]error{nil}, nil).AnyTimes()

	client := s.getSecretsClient()
	err = client.UpdateAccessSecret(s.T().Context(), &GrantRevokeAccessSecretInput{
		SecretId:     secretId,
		ModelUUID:    *s.testModelName,
		Applications: applications,
	}, GrantAccess)
	s.Require().NoError(err)

	err = client.UpdateAccessSecret(s.T().Context(), &GrantRevokeAccessSecretInput{
		SecretId:     secretId,
		ModelUUID:    *s.testModelName,
		Applications: applications,
	}, RevokeAccess)
	s.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestUserSecretSuite(t *testing.T) {
	suite.Run(t, new(SecretSuite))
}
