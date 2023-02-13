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

type CreateSSHKeysInput struct {
	ModelName string
	ModelUUID string
	Keys      []string
}

type ReadSSHKeysInput struct {
	ModelName string
	ModelUUID string
}

type ReadSSHKeysOutput struct {
	ModelName string
	Keys      []string
}

type DeleteSSHKeysInput struct {
	ModelName string
	ModelUUID string
	Keys      []string
}

func newSSHKeysClient(cf ConnectionFactory) *sshKeysClient {
	return &sshKeysClient{
		ConnectionFactory: cf,
	}
}

func (c *sshKeysClient) CreateSSHKeys(input *CreateSSHKeysInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	client := keymanager.NewClient(conn)
	defer client.Close()

	// NOTE
	// Juju only stores ssh keys at a global level.
	params, err := client.AddKeys("admin", input.Keys...)
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

func (c *sshKeysClient) ReadSSHKeys(input *ReadSSHKeysInput) (*ReadSSHKeysOutput, error) {
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

	return &ReadSSHKeysOutput{
		ModelName: input.ModelName,
		Keys:      keys}, nil
}

func (c *sshKeysClient) DeleteSSHKeys(input *DeleteSSHKeysInput) error {
	conn, err := c.GetConnection(&input.ModelUUID)
	if err != nil {
		return err
	}

	client := keymanager.NewClient(conn)
	defer client.Close()

	// NOTE: Right now Juju uses global users for keys
	// find the user of the key
	users := make([]string, len(input.Keys))
	for i, k := range input.Keys {
		users[i] = getUserFromSSHKey(k)
	}
	params, err := client.DeleteKeys("admin", users...)
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
