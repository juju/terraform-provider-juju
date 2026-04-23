// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"fmt"
	"sync"

	"github.com/juju/juju/api/client/keymanager"
	"github.com/juju/utils/v4/ssh"
)

type sshKeysClient struct {
	SharedClient

	// KeyLock is used to prevent concurrent calls to AddKeys, ReadKeys and DeleteKeys
	// which can lead to race conditions in Juju. Issue: https://github.com/juju/juju/issues/20447
	KeyLock *sync.RWMutex
}

// CreateSSHKeyInput contains the parameters for creating an SSH key.
type CreateSSHKeyInput struct {
	Username  string
	ModelUUID string
	Payload   string
}

// ReadSSHKeyInput contains the parameters for reading an SSH key.
type ReadSSHKeyInput struct {
	Username      string
	ModelUUID     string
	KeyIdentifier string
}

// ReadSSHKeyOutput contains the SSH key payload.
type ReadSSHKeyOutput struct {
	Payload string
}

// DeleteSSHKeyInput contains the parameters for deleting an SSH key.
type DeleteSSHKeyInput struct {
	Username      string
	ModelUUID     string
	KeyIdentifier string
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

	client := keymanager.NewClient(conn)

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
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	returnedKeys, err := client.ListKeys(ctx, ssh.FullKeys, input.Username)
	if err != nil {
		return nil, err
	}

	for _, res := range returnedKeys {
		for _, k := range res.Result {
			fingerprint, comment, err := ssh.KeyFingerprint(k)
			if err != nil {
				return nil, fmt.Errorf("error getting fingerprint for ssh key: %w", err)
			}
			if input.KeyIdentifier == fingerprint || input.KeyIdentifier == comment {
				return &ReadSSHKeyOutput{
					Payload: k,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no ssh key found for %s", input.KeyIdentifier)
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

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	params, err := client.DeleteKeys(ctx, input.Username, input.KeyIdentifier)
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

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	results, err := client.ListKeys(ctx, ssh.FullKeys, input.Username)
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
