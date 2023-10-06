// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"

	"github.com/juju/juju/api/client/keymanager"
	"github.com/juju/utils/v3/ssh"

	"github.com/juju/terraform-provider-juju/internal/utils"
)

type sshKeysClient struct {
	SharedClient
}

type CreateSSHKeyInput struct {
	ModelName string
	Payload   string
}

type ReadSSHKeyInput struct {
	ModelName     string
	KeyIdentifier string
}

type ReadSSHKeyOutput struct {
	ModelName string
	Payload   string
}

type DeleteSSHKeyInput struct {
	ModelName     string
	KeyIdentifier string
}

func newSSHKeysClient(sc SharedClient) *sshKeysClient {
	return &sshKeysClient{
		SharedClient: sc,
	}
}

func (c *sshKeysClient) CreateSSHKey(input *CreateSSHKeyInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE
	// Juju only stores ssh keys at a global level.
	params, err := client.AddKeys("admin", input.Payload)
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
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := keymanager.NewClient(conn)

	// NOTE: At this moment Juju only uses global ssh keys.
	// We hardcode the user to be admin.
	returnedKeys, err := client.ListKeys(ssh.FullKeys, "admin")
	if err != nil {
		return nil, err
	}

	for _, res := range returnedKeys {
		for _, k := range res.Result {
			if input.KeyIdentifier == utils.GetKeyIdentifierFromSSHKey(k) {
				return &ReadSSHKeyOutput{
					ModelName: input.ModelName,
					Payload:   k,
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no ssh key found for %s", input.KeyIdentifier)
}

func (c *sshKeysClient) DeleteSSHKey(input *DeleteSSHKeyInput) error {
	conn, err := c.GetConnection(&input.ModelName)
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
	returnedKeys, err := client.ListKeys(ssh.FullKeys, "admin")
	if err != nil {
		return err
	}
	// only check if there is one key
	if len(returnedKeys) == 1 {
		k := returnedKeys[0].Result[0]
		if input.KeyIdentifier == utils.GetKeyIdentifierFromSSHKey(k) {
			// This is the latest key, do not remove it
			c.Warnf(fmt.Sprintf("ssh key from user %s is the last one and will not be removed", input.KeyIdentifier))
			return nil
		}
	}

	// NOTE: Right now Juju uses global users for keys
	params, err := client.DeleteKeys("admin", input.KeyIdentifier)
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
