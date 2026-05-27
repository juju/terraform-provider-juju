// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/juju/juju/api"
	"github.com/juju/juju/core/semversion"
	"github.com/juju/juju/rpc/params"
	jujussh "github.com/juju/utils/v4/ssh"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
	gossh "golang.org/x/crypto/ssh"
)

type SSHKeysSuite struct {
	suite.Suite
	JujuSuite

	mockSSHKeyManagerClient *MockSSHKeyManagerClient
}

func (s *SSHKeysSuite) SetupSuite() {
	modelUUID := "test-model-uuid"
	s.testModelName = &modelUUID
}

func (s *SSHKeysSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := s.JujuSuite.setupMocks(t)
	s.mockSSHKeyManagerClient = NewMockSSHKeyManagerClient(ctlr)
	return ctlr
}

func (s *SSHKeysSuite) getSSHKeysClient() sshKeysClient {
	return sshKeysClient{
		SharedClient: s.mockSharedClient,
		KeyLock:      &sync.RWMutex{},
		getKeyManagerClient: func(api.Connection) SSHKeyManagerClient {
			return s.mockSSHKeyManagerClient
		},
	}
}

func (s *SSHKeysSuite) TestDeleteSSHKeyUsesMD5FingerprintOnJuju3() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	payload := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere\n"
	s.mockSharedClient.EXPECT().IsJAAS(gomock.Any(), false).Return(false)
	s.mockSharedClient.EXPECT().GetControllerVersion(gomock.Any()).Return(semversion.MustParse("3.6.10"), nil)
	expectedFingerprint, _, err := jujussh.KeyFingerprint("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere")
	s.Require().NoError(err)
	s.mockSSHKeyManagerClient.EXPECT().DeleteKeys(gomock.Any(), "admin", expectedFingerprint).Return(nil, nil)

	client := s.getSSHKeysClient()
	err = client.DeleteSSHKey(context.Background(), &DeleteSSHKeyInput{
		Username:  "admin",
		ModelUUID: *s.testModelName,
		Payload:   payload,
	})
	s.Require().NoError(err)
}

func (s *SSHKeysSuite) TestDeleteSSHKeyUsesSHA256FingerprintOnJuju4() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	payload := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere"
	s.mockSharedClient.EXPECT().IsJAAS(gomock.Any(), false).Return(false)
	s.mockSharedClient.EXPECT().GetControllerVersion(gomock.Any()).Return(semversion.MustParse("4.0.2"), nil)
	publicKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(payload))
	s.Require().NoError(err)
	s.mockSSHKeyManagerClient.EXPECT().DeleteKeys(gomock.Any(), "admin", gossh.FingerprintSHA256(publicKey)).Return(nil, nil)

	client := s.getSSHKeysClient()
	err = client.DeleteSSHKey(context.Background(), &DeleteSSHKeyInput{
		Username:  "admin",
		ModelUUID: *s.testModelName,
		Payload:   payload,
	})
	s.Require().NoError(err)
}

func (s *SSHKeysSuite) TestDeleteSSHKeyUsesBothHashesOnJIMM() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	payload := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere"
	s.mockSharedClient.EXPECT().IsJAAS(gomock.Any(), false).Return(true)

	expectedMD5, _, err := jujussh.KeyFingerprint(payload)
	s.Require().NoError(err)
	publicKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(payload))
	s.Require().NoError(err)
	expectedSHA256 := gossh.FingerprintSHA256(publicKey)

	s.mockSSHKeyManagerClient.EXPECT().DeleteKeys(gomock.Any(), "admin", expectedMD5, expectedSHA256).Return(nil, nil)

	client := s.getSSHKeysClient()
	err = client.DeleteSSHKey(context.Background(), &DeleteSSHKeyInput{
		Username:  "admin",
		ModelUUID: *s.testModelName,
		Payload:   payload,
	})
	s.Require().NoError(err)
}

func (s *SSHKeysSuite) TestDeleteSSHKeyAggregatesErrorsForBothHashes() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	payload := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAID3gjJTJtYZU55HTUr+hu0JF9p152yiC9czJi9nKojuW jimmy@somewhere"
	s.mockSharedClient.EXPECT().IsJAAS(gomock.Any(), false).Return(true)

	expectedMD5, _, err := jujussh.KeyFingerprint(payload)
	s.Require().NoError(err)
	publicKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(payload))
	s.Require().NoError(err)
	expectedSHA256 := gossh.FingerprintSHA256(publicKey)

	s.mockSSHKeyManagerClient.EXPECT().DeleteKeys(gomock.Any(), "admin", expectedMD5, expectedSHA256).Return([]params.ErrorResult{{
		Error: &params.Error{Message: "md5 failed"},
	}, {
		Error: &params.Error{Message: "sha256 failed"},
	}}, nil)

	client := s.getSSHKeysClient()
	err = client.DeleteSSHKey(context.Background(), &DeleteSSHKeyInput{
		Username:  "admin",
		ModelUUID: *s.testModelName,
		Payload:   payload,
	})
	s.Require().Error(err)
	s.Equal(fmt.Sprintf("[%s %s]", "md5 failed", "sha256 failed"), err.Error())
}

func TestSSHKeysSuite(t *testing.T) {
	suite.Run(t, new(SSHKeysSuite))
}
