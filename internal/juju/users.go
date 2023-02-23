package juju

import (
	"fmt"

	"github.com/juju/juju/api/client/usermanager"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type usersClient struct {
	ConnectionFactory
}

type CreateUserInput struct {
	Name        string
	DisplayName string
	Model       string
	Password    string
}

type CreateUserResponse struct {
	UserTag names.UserTag
	Secret  []byte
}

type ReadUserInput struct {
	Name string
}

type ReadUserResponse struct {
	UserInfo params.UserInfo
}

type ReadModelUserResponse struct {
	ModelUserInfo []params.ModelUserInfo
}

type UpdateUserInput struct {
	Name        string
	DisplayName string
	User        string
	Password    string
}

type DestroyUserInput struct {
	Name string
}

func newUsersClient(cf ConnectionFactory) *usersClient {
	return &usersClient{
		ConnectionFactory: cf,
	}
}

func (c *usersClient) CreateUser(input CreateUserInput) (*CreateUserResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	client := usermanager.NewClient(conn)
	defer client.Close()

	userTag, userSecret, err := client.AddUser(input.Name, input.DisplayName, input.Password)
	if err != nil {
		return nil, err
	}

	return &CreateUserResponse{UserTag: userTag, Secret: userSecret}, nil
}

func (c *usersClient) ReadUser(name string) (*ReadUserResponse, error) {
	usermanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	usermanagerClient := usermanager.NewClient(usermanagerConn)
	defer usermanagerClient.Close()

	users, err := usermanagerClient.UserInfo([]string{name}, false) //don't list disabled users
	if err != nil {
		return nil, err
	}

	if len(users) > 1 {
		return nil, fmt.Errorf("more than one user returned for user name: %s", name)
	}
	if len(users) < 1 {
		return nil, fmt.Errorf("no user returned for user name: %s", name)
	}

	userInfo := users[0]

	return &ReadUserResponse{
		UserInfo: userInfo,
	}, nil
}

func (c *usersClient) ModelUserInfo(uuid string) (*ReadModelUserResponse, error) {
	usermanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}

	usermanagerClient := usermanager.NewClient(usermanagerConn)
	defer usermanagerClient.Close()

	users, err := usermanagerClient.ModelUserInfo(uuid)
	if err != nil {
		return nil, err
	}

	if len(users) < 1 {
		return nil, fmt.Errorf("no users returned for model name: %s", uuid)
	}

	return &ReadModelUserResponse{
		ModelUserInfo: users,
	}, nil
}

func (c *usersClient) UpdateUser(input UpdateUserInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := usermanager.NewClient(conn)
	defer client.Close()

	if input.Password != "" {
		err = client.SetPassword(input.Name, input.Password)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *usersClient) DestroyUser(input DestroyUserInput) error {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return err
	}

	client := usermanager.NewClient(conn)
	defer client.Close()

	err = client.RemoveUser(input.Name)
	if err != nil {
		return err
	}

	return nil
}
