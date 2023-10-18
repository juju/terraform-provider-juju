// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"fmt"

	"github.com/juju/juju/api/client/usermanager"
	"github.com/juju/juju/rpc/params"
	"github.com/juju/names/v4"
)

type usersClient struct {
	SharedClient
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

func newUsersClient(sc SharedClient) *usersClient {
	return &usersClient{
		SharedClient: sc,
	}
}

func (c *usersClient) CreateUser(input CreateUserInput) (*CreateUserResponse, error) {
	conn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := usermanager.NewClient(conn)

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
	defer func() { _ = usermanagerConn.Close() }()

	usermanagerClient := usermanager.NewClient(usermanagerConn)

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

func (c *usersClient) ModelUserInfo(modelName string) (*ReadModelUserResponse, error) {
	usermanagerConn, err := c.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = usermanagerConn.Close() }()
	usermanagerClient := usermanager.NewClient(usermanagerConn)

	uuid, err := c.ModelUUID(modelName)
	if err != nil {
		return nil, err
	}

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
	defer func() { _ = conn.Close() }()

	client := usermanager.NewClient(conn)

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
	defer func() { _ = conn.Close() }()

	client := usermanager.NewClient(conn)

	err = client.RemoveUser(input.Name)
	if err != nil {
		return err
	}

	return nil
}
