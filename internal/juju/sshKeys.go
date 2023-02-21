package juju

import (
	"fmt"
	"strings"

	"github.com/juju/juju/api/client/keymanager"
	"github.com/juju/utils/v3/ssh"
)

type sshKeysClient struct {
	ConnectionFactory
}

type CreateSSHKeyInput struct {
	ModelName string
	ModelUUID string
	Payload   string
}

type ReadSSHKeyInput struct {
	ModelName string
	ModelUUID string
	User      string
}

type ReadSSHKeyOutput struct {
	ModelName string
	Payload   string
}

type DeleteSSHKeyInput struct {
	ModelName string
	ModelUUID string
	User      string
}

func newSSHKeysClient(cf ConnectionFactory) *sshKeysClient {
	return &sshKeysClient{
		ConnectionFactory: cf,
	}
}

func (c *sshKeysClient) CreateSSHKey(input *CreateSSHKeyInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	client := keymanager.NewClient(conn)
	defer client.Close()

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
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return nil, err
	}

	client := keymanager.NewClient(conn)
	defer client.Close()

	// NOTE: At this moment Juju only uses global ssh keys.
	// We hardcode the user to be admin.
	returnedKeys, err := client.ListKeys(ssh.FullKeys, "admin")
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0)
	keys = append(keys, returnedKeys[0].Result...)

	for _, k := range keys {
		if input.User == getUserFromSSHKey(k) {
			return &ReadSSHKeyOutput{
				ModelName: input.ModelName,
				Payload:   k,
			}, nil
		}
	}

	return nil, fmt.Errorf("no ssh key found for %s", input.User)
}

func (c *sshKeysClient) DeleteSSHKey(input *DeleteSSHKeyInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	client := keymanager.NewClient(conn)
	defer client.Close()

	// NOTE: Right now Juju uses global users for keys
	params, err := client.DeleteKeys("admin", input.User)
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

// getUserFromSSHKey returns the user of the key
// returning the string after the = symbol
func getUserFromSSHKey(key string) string {
	end := strings.LastIndex(key, "=")
	if end < 0 {
		return ""
	}
	return key[end+2:]
}
