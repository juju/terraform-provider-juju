// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"strings"
	"sync"

	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/keymanager"
	"github.com/juju/juju/core/semversion"
	jujussh "github.com/juju/utils/v4/ssh"
	gossh "golang.org/x/crypto/ssh"
)

// NewSSHKeyNotFoundError returns a new error indicating that the SSH key was not found.
func NewSSHKeyNotFoundError(keyIdentifier string) error {
	return jujuerrors.WithType(jujuerrors.Errorf("ssh key %s not found", keyIdentifier), SSHKeyNotFoundError)
}

// SSHKeyNotFoundError indicates that the SSH key was not found when contacting the Juju API.
var SSHKeyNotFoundError = jujuerrors.ConstError("ssh-key-not-found")

type sshKeysClient struct {
	SharedClient

	// KeyLock is used to prevent concurrent calls to AddKeys, ReadKeys and DeleteKeys
	// which can lead to race conditions in Juju. Issue: https://github.com/juju/juju/issues/20447
	KeyLock *sync.RWMutex

	getKeyManagerClient func(api.Connection) SSHKeyManagerClient
}

// CreateSSHKeyInput contains the parameters for creating an SSH key.
type CreateSSHKeyInput struct {
	Username  string
	ModelUUID string
	Payload   string
}

// ReadSSHKeyInput contains the parameters for reading an SSH key.
type ReadSSHKeyInput struct {
	Username  string
	ModelUUID string
	Payload   string
}

// ReadSSHKeyOutput contains the SSH key payload.
type ReadSSHKeyOutput struct {
	Payload string
}

// DeleteSSHKeyInput contains the parameters for deleting an SSH key.
type DeleteSSHKeyInput struct {
	Username  string
	ModelUUID string
	Payload   string
}

// ListSSHKeysInput is the input for ListSSHKeys.
type ListSSHKeysInput struct {
	Username  string
	ModelUUID string
}

// ListSSHKeysOutput is the output for ListSSHKeys.
type ListSSHKeysOutput struct {
	Payloads []string
}

func newSSHKeysClient(sc SharedClient) *sshKeysClient {
	return &sshKeysClient{
		SharedClient: sc,
		KeyLock:      &sync.RWMutex{},
		getKeyManagerClient: func(conn api.Connection) SSHKeyManagerClient {
			return keymanager.NewClient(conn)
		},
	}
}

// CreateSSHKey adds an SSH key to the specified model.
func (c *sshKeysClient) CreateSSHKey(ctx context.Context, input *CreateSSHKeyInput) error {
	c.KeyLock.Lock()
	defer c.KeyLock.Unlock()
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := c.getKeyManagerClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	params, err := client.AddKeys(ctx, input.Username, input.Payload)
	if err != nil {
		return err
	}
	if len(params) != 0 {
		messages := make([]string, 0)
		for _, e := range params {
			if e.Error != nil {
				messages = append(messages, e.Error.Message)
			}
		}
		if len(messages) == 0 {
			return nil
		}
		err = fmt.Errorf("%s", messages)
		return err
	}

	return nil
}

// ReadSSHKey retrieves an SSH key by identifier.
func (c *sshKeysClient) ReadSSHKey(ctx context.Context, input *ReadSSHKeyInput) (*ReadSSHKeyOutput, error) {
	c.KeyLock.RLock()
	defer c.KeyLock.RUnlock()
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys are associated with users.
	returnedKeys, err := client.ListKeys(ctx, jujussh.FullKeys, input.Username)
	if err != nil {
		return nil, err
	}

	inputKey, _, err := jujussh.KeyFingerprint(input.Payload)
	if err != nil {
		return nil, fmt.Errorf("error getting fingerprint for input ssh key: %w", err)
	}

	for _, res := range returnedKeys {
		for _, k := range res.Result {
			fingerprint, _, err := jujussh.KeyFingerprint(k)
			if err != nil {
				return nil, fmt.Errorf("error getting fingerprint for ssh key: %w", err)
			}
			if inputKey == fingerprint {
				return &ReadSSHKeyOutput{
					Payload: k,
				}, nil
			}
		}
	}

	return nil, NewSSHKeyNotFoundError(input.Payload)
}

// DeleteSSHKey removes an SSH key from the specified model.
func (c *sshKeysClient) DeleteSSHKey(ctx context.Context, input *DeleteSSHKeyInput) error {
	c.KeyLock.Lock()
	defer c.KeyLock.Unlock()
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := c.getKeyManagerClient(conn)
	isJIMMController := c.IsJAAS(ctx, false)

	controllerVersion := semversion.Number{}
	if !isJIMMController {
		controllerVersion, err = c.GetControllerVersion(ctx)
		if err != nil {
			return err
		}
	}

	deleteKeyIdentifiers, err := sshKeyDeleteIdentifiers(input.Payload, controllerVersion, isJIMMController)
	if err != nil {
		return fmt.Errorf("generating delete identifiers for ssh key: %w", err)
	}

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4, keys are associated per user per model, but note, it isn't the user passed in
	// via the user argument but rather the API authenticated user.
	params, err := client.DeleteKeys(ctx, input.Username, deleteKeyIdentifiers...)
	if len(params) != 0 {
		messages := make([]string, 0)
		for _, e := range params {
			if e.Error != nil {
				messages = append(messages, e.Error.Message)
			}
		}
		if len(messages) == 0 {
			return nil
		}
		err = fmt.Errorf("%s", messages)
		return err
	}

	return err
}

// ListKeys returns the authorised ssh keys for the specified users.
func (c *sshKeysClient) ListKeys(ctx context.Context, input ListSSHKeysInput) ([]string, error) {
	conn, err := c.GetConnection(ctx, &input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := c.getKeyManagerClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	results, err := client.ListKeys(ctx, jujussh.FullKeys, input.Username)
	if err != nil {
		return nil, err
	}

	// Incase this looks strange, the Juju CLI does the same and takes [0].
	result := results[0]
	if result.Error != nil {
		return nil, result.Error
	}

	return result.Result, nil
}

// sshKeyDeleteIdentifiers generates a list of identifiers to attempt to delete the SSH key with, based on the
// controller version and whether the controller is JIMM/JAAS-backed.
//
// Note that JIMM fronts multiple controller versions and reports the oldest attached controller version.
// Because the KeyManager facade was not bumped when a breaking change was made to accept SHA256 fingerprints in Juju 4
// over MD5 in Juju 3 and because DeleteKeys is a model-scoped call, when JIMM is in front we send both hash formats in
// one request and let the backing controller accept the identifier format it understands.
func sshKeyDeleteIdentifiers(payload string, controllerVersion semversion.Number, isJIMMController bool) ([]string, error) {
	normalizedPayload := strings.TrimSuffix(payload, "\n")
	md5Fingerprint, _, err := jujussh.KeyFingerprint(normalizedPayload)
	if err != nil {
		return nil, err
	}

	publicKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(normalizedPayload))
	if err != nil {
		return nil, err
	}
	sha256Fingerprint := gossh.FingerprintSHA256(publicKey)

	if isJIMMController {
		return []string{md5Fingerprint, sha256Fingerprint}, nil
	}
	if controllerVersion.Major >= 4 {
		return []string{sha256Fingerprint}, nil
	}
	return []string{md5Fingerprint}, nil
}
