// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"
	"sync"

	"github.com/juju/juju/api/client/keymanager"
	"github.com/juju/utils/v3/ssh"
)

type sshKeysClient struct {
	SharedClient

	// KeyLock is used to prevent concurrent calls to AddKeys, ReadKeys and DeleteKeys
	// which can lead to race conditions in Juju. Issue: https://github.com/juju/juju/issues/20447
	KeyLock *sync.RWMutex
}

type CreateSSHKeyInput struct {
	Username  string
	ModelUUID string
	Payload   string
}

type ReadSSHKeyInput struct {
	Username      string
	ModelUUID     string
	KeyIdentifier string
}

type ReadSSHKeyOutput struct {
	Payload string
}

type DeleteSSHKeyInput struct {
	Username      string
	ModelUUID     string
	KeyIdentifier string
}

type ListSSHKeysInput struct {
	Username  string
	ModelUUID string
}

type ListSSHKeysOutput struct {
	Payloads []string
}

func newSSHKeysClient(sc SharedClient) *sshKeysClient {
	return &sshKeysClient{
		SharedClient: sc,
		KeyLock:      &sync.RWMutex{},
	}
}

func (c *sshKeysClient) CreateSSHKey(input *CreateSSHKeyInput) error {
	c.KeyLock.Lock()
	defer c.KeyLock.Unlock()
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	params, err := client.AddKeys(input.Username, input.Payload)
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

func (c *sshKeysClient) ReadSSHKey(input *ReadSSHKeyInput) (*ReadSSHKeyOutput, error) {
	c.KeyLock.RLock()
	defer c.KeyLock.RUnlock()
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	returnedKeys, err := client.ListKeys(ssh.FullKeys, input.Username)
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

func (c *sshKeysClient) DeleteSSHKey(input *DeleteSSHKeyInput) error {
	c.KeyLock.Lock()
	defer c.KeyLock.Unlock()
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: Unfortunately Juju will return an error if we try to
	// remove the last ssh key from the controller. This is something
	// that impacts the current Juju logic. As a temporal workaround
	// we will check if this is the latest SSH key of this model and
	// skip the delete.
	returnedKeys, err := client.ListKeys(ssh.FullKeys, input.Username)
	if err != nil {
		return err
	}
	// only check if there is one key
	if len(returnedKeys) == 1 {
		fingerprint, comment, err := ssh.KeyFingerprint(returnedKeys[0].Result[0])
		if err != nil {
			return fmt.Errorf("error getting fingerprint for ssh key: %w", err)
		}
		if input.KeyIdentifier == fingerprint || input.KeyIdentifier == comment {
			// This is the latest key, do not remove it
			c.Warnf(fmt.Sprintf("ssh key from user %s is the last one and will not be removed", input.KeyIdentifier))
			return nil
		}
	}

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.<
	params, err := client.DeleteKeys(input.Username, input.KeyIdentifier)
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
func (c *sshKeysClient) ListKeys(input ListSSHKeysInput) ([]string, error) {
	c.KeyLock.Lock()
	defer c.KeyLock.Unlock()
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: In Juju 3.6 ssh keys are not associated with user - they are global per model. We pass in
	// the logged-in user for completeness. In Juju 4 ssh keys will be associated with users.
	results, err := client.ListKeys(ssh.FullKeys, input.Username)
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
