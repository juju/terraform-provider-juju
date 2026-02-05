// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"errors"
	"testing"

	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/juju/juju/api"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type JaasSuite struct {
	suite.Suite
	JujuSuite

	mockJaasClient *MockJaasAPIClient
}

func (s *JaasSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := s.JujuSuite.setupMocks(t)
	s.mockJaasClient = NewMockJaasAPIClient(ctlr)

	return ctlr
}

func (s *JaasSuite) getJaasClient() jaasClient {
	return jaasClient{
		SharedClient: s.mockSharedClient,
		getJaasApiClient: func(connection api.Connection) JaasAPIClient {
			return s.mockJaasClient
		},
	}
}

func (s *JaasSuite) TestAddRelations() {
	defer s.setupMocks(s.T()).Finish()

	tuples := []JaasTuple{
		{Object: "object-1", Relation: "relation", Target: "target-1"},
		{Object: "object-2", Relation: "relation", Target: "target-2"},
	}
	req := params.AddRelationRequest{
		Tuples: toAPITuples(tuples),
	}

	s.mockJaasClient.EXPECT().AddRelation(
		&req,
	).Return(nil)

	client := s.getJaasClient()
	err := client.AddRelations(tuples)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestAddRelationsEmptySlice() {
	expectedErr := errors.New("empty slice of tuples")
	client := s.getJaasClient()
	err := client.AddRelations([]JaasTuple{})
	s.Require().Error(err)
	s.Assert().Equal(expectedErr, err)
}

func (s *JaasSuite) TestDeleteRelations() {
	defer s.setupMocks(s.T()).Finish()

	tuples := []JaasTuple{
		{Object: "object-1", Relation: "relation", Target: "target-1"},
		{Object: "object-2", Relation: "relation", Target: "target-2"},
	}
	req := params.RemoveRelationRequest{
		Tuples: toAPITuples(tuples),
	}

	s.mockJaasClient.EXPECT().RemoveRelation(
		&req,
	).Return(nil)

	client := s.getJaasClient()
	err := client.DeleteRelations(tuples)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestDeleteRelationsEmptySlice() {
	expectedErr := errors.New("empty slice of tuples")
	client := s.getJaasClient()
	err := client.DeleteRelations([]JaasTuple{})
	s.Require().Error(err)
	s.Assert().Equal(expectedErr, err)
}

func (s *JaasSuite) TestReadRelations() {
	defer s.setupMocks(s.T()).Finish()

	tuple := JaasTuple{Object: "object-1", Relation: "relation", Target: "target-1"}
	// 1st request/response has no token in the request and a token in the response indicating another page is available.
	req := &params.ListRelationshipTuplesRequest{Tuple: toAPITuple(tuple)}
	respWithToken := &params.ListRelationshipTuplesResponse{
		Tuples:            []params.RelationshipTuple{toAPITuple(tuple)},
		ContinuationToken: "token",
	}
	s.mockJaasClient.EXPECT().ListRelationshipTuples(
		req,
	).Return(respWithToken, nil)
	// 2nd request/response has the previous token in the request and no token in the response, indicating all pages have been consumed.
	reqWithToken := &params.ListRelationshipTuplesRequest{Tuple: toAPITuple(tuple), ContinuationToken: "token"}
	respWithoutToken := &params.ListRelationshipTuplesResponse{
		Tuples:            []params.RelationshipTuple{toAPITuple(tuple)},
		ContinuationToken: "",
	}
	s.mockJaasClient.EXPECT().ListRelationshipTuples(
		reqWithToken,
	).Return(respWithoutToken, nil)

	client := s.getJaasClient()
	relations, err := client.ReadRelations(context.Background(), &tuple)
	s.Require().NoError(err)
	s.Require().Len(relations, 2)
	s.Require().Equal(relations, []JaasTuple{tuple, tuple})
}

func (s *JaasSuite) TestReadRelationsEmptyTuple() {
	expectedErr := errors.New("read relation tuple is nil")
	client := s.getJaasClient()
	_, err := client.ReadRelations(context.Background(), nil)
	s.Require().Error(err)
	s.Assert().Equal(expectedErr, err)
}

func (s *JaasSuite) TestReadRelationsCancelledContext() {
	defer s.setupMocks(s.T()).Finish()

	tuple := JaasTuple{Object: "object-1", Relation: "relation", Target: "target-1"}
	req := &params.ListRelationshipTuplesRequest{Tuple: toAPITuple(tuple)}
	respWithToken := &params.ListRelationshipTuplesResponse{
		Tuples:            []params.RelationshipTuple{toAPITuple(tuple)},
		ContinuationToken: "token",
	}
	s.mockJaasClient.EXPECT().ListRelationshipTuples(req).Return(respWithToken, nil)

	expectedErr := errors.New("context canceled")
	ctx := context.Background()
	ctx, cancelFunc := context.WithCancel(ctx)
	cancelFunc()

	client := s.getJaasClient()
	_, err := client.ReadRelations(ctx, &tuple)
	s.Require().Error(err)
	s.Assert().Equal(expectedErr, err)
}

func (s *JaasSuite) TestAddGroup() {
	defer s.setupMocks(s.T()).Finish()

	name := "group"
	req := &params.AddGroupRequest{Name: name}
	resp := params.AddGroupResponse{Group: params.Group{UUID: "uuid", Name: name}}

	s.mockJaasClient.EXPECT().AddGroup(req).Return(resp, nil)

	client := s.getJaasClient()
	uuid, err := client.AddGroup(name)
	s.Require().NoError(err)
	s.Require().Equal(resp.UUID, uuid)
}

func (s *JaasSuite) TestGetGroup() {
	defer s.setupMocks(s.T()).Finish()

	uuid := "uuid"
	name := "group"

	req := &params.GetGroupRequest{UUID: uuid}
	resp := params.GetGroupResponse{Group: params.Group{UUID: uuid, Name: name}}
	s.mockJaasClient.EXPECT().GetGroup(req).Return(resp, nil)

	client := s.getJaasClient()
	gotGroup, err := client.ReadGroupByUUID(uuid)
	s.Require().NoError(err)
	s.Require().Equal(*gotGroup, JaasGroup{UUID: uuid, Name: name})
}

func (s *JaasSuite) TestGetGroupNotFound() {
	defer s.setupMocks(s.T()).Finish()

	uuid := "uuid"

	req := &params.GetGroupRequest{UUID: uuid}
	s.mockJaasClient.EXPECT().GetGroup(req).Return(params.GetGroupResponse{}, errors.New("group not found"))

	client := s.getJaasClient()
	gotGroup, err := client.ReadGroupByUUID(uuid)
	s.Require().Error(err)
	s.Require().Nil(gotGroup)
}

func (s *JaasSuite) TestRenameGroup() {
	defer s.setupMocks(s.T()).Finish()

	name := "name"
	newName := "new-name"
	req := &params.RenameGroupRequest{Name: name, NewName: newName}
	s.mockJaasClient.EXPECT().RenameGroup(req).Return(nil)

	client := s.getJaasClient()
	err := client.RenameGroup(name, newName)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestRemoveGroup() {
	defer s.setupMocks(s.T()).Finish()

	name := "group"
	req := &params.RemoveGroupRequest{Name: name}
	s.mockJaasClient.EXPECT().RemoveGroup(req).Return(nil)

	client := s.getJaasClient()
	err := client.RemoveGroup(name)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestAddRole() {
	defer s.setupMocks(s.T()).Finish()

	name := "role"
	req := &params.AddRoleRequest{Name: name}
	resp := params.AddRoleResponse{Role: params.Role{UUID: "uuid", Name: name}}

	s.mockJaasClient.EXPECT().AddRole(req).Return(resp, nil)

	client := s.getJaasClient()
	uuid, err := client.AddRole(name)
	s.Require().NoError(err)
	s.Require().Equal(resp.UUID, uuid)
}

func (s *JaasSuite) TestGetRole() {
	defer s.setupMocks(s.T()).Finish()

	uuid := "uuid"
	name := "role"

	req := &params.GetRoleRequest{UUID: uuid}
	resp := params.GetRoleResponse{Role: params.Role{UUID: uuid, Name: name}}
	s.mockJaasClient.EXPECT().GetRole(req).Return(resp, nil)

	client := s.getJaasClient()
	gotRole, err := client.ReadRoleByUUID(uuid)
	s.Require().NoError(err)
	s.Require().Equal(*gotRole, JaasRole{UUID: uuid, Name: name})
}

func (s *JaasSuite) TestGetRoleNotFound() {
	defer s.setupMocks(s.T()).Finish()

	uuid := "uuid"

	req := &params.GetRoleRequest{UUID: uuid}
	s.mockJaasClient.EXPECT().GetRole(req).Return(params.GetRoleResponse{}, errors.New("role not found"))

	client := s.getJaasClient()
	gotRole, err := client.ReadRoleByUUID(uuid)
	s.Require().Error(err)
	s.Require().Nil(gotRole)
}

func (s *JaasSuite) TestRenameRole() {
	defer s.setupMocks(s.T()).Finish()

	name := "name"
	newName := "new-name"
	req := &params.RenameRoleRequest{Name: name, NewName: newName}
	s.mockJaasClient.EXPECT().RenameRole(req).Return(nil)

	client := s.getJaasClient()
	err := client.RenameRole(name, newName)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestRemoveRole() {
	defer s.setupMocks(s.T()).Finish()

	name := "role"
	req := &params.RemoveRoleRequest{Name: name}
	s.mockJaasClient.EXPECT().RemoveRole(req).Return(nil)

	client := s.getJaasClient()
	err := client.RemoveRole(name)
	s.Require().NoError(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestJaasSuite(t *testing.T) {
	suite.Run(t, new(JaasSuite))
}
