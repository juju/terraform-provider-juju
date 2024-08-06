// Copyright 2023 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
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

func (s *JaasSuite) SetupTest() {}

func (s *JaasSuite) setupMocks(t *testing.T) *gomock.Controller {
	ctlr := s.JujuSuite.setupMocks(t, nil)
	s.mockJaasClient = NewMockJaasAPIClient(ctlr)

	return ctlr
}

func (s *JaasSuite) getJaasClient() jaasClient {
	return jaasClient{
		SharedClient: s.JujuSuite.mockSharedClient,
		getJaasApiClient: func(connection api.Connection) JaasAPIClient {
			return s.mockJaasClient
		},
	}
}

func (s *JaasSuite) TestAddRelations() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	tuples := []params.RelationshipTuple{
		{Object: "object-1", Relation: "relation", TargetObject: "target-1"},
		{Object: "object-2", Relation: "relation", TargetObject: "target-2"},
	}
	req := params.AddRelationRequest{
		Tuples: tuples,
	}

	s.mockJaasClient.EXPECT().AddRelation(
		&req,
	).Return(nil).Times(1)

	client := s.getJaasClient()
	err := client.AddRelations(tuples)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestDeleteRelations() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	tuples := []params.RelationshipTuple{
		{Object: "object-1", Relation: "relation", TargetObject: "target-1"},
		{Object: "object-2", Relation: "relation", TargetObject: "target-2"},
	}
	req := params.RemoveRelationRequest{
		Tuples: tuples,
	}

	s.mockJaasClient.EXPECT().RemoveRelation(
		&req,
	).Return(nil).Times(1)

	client := s.getJaasClient()
	err := client.DeleteRelations(tuples)
	s.Require().NoError(err)
}

func (s *JaasSuite) TestReadRelations() {
	ctlr := s.setupMocks(s.T())
	defer ctlr.Finish()

	tuple := params.RelationshipTuple{Object: "object-1", Relation: "relation", TargetObject: "target-1"}
	// 1st request/response has no token in the request and a token in the response indicating another page is available.
	req := &params.ListRelationshipTuplesRequest{Tuple: tuple}
	respWithToken := &params.ListRelationshipTuplesResponse{
		Tuples:            []params.RelationshipTuple{tuple},
		ContinuationToken: "token",
	}
	s.mockJaasClient.EXPECT().ListRelationshipTuples(
		req,
	).Return(respWithToken, nil).Times(1)
	// 2nd request/response has the previous token in the request and no token in the response, indicating all pages have been consumed.
	reqWithToken := &params.ListRelationshipTuplesRequest{Tuple: tuple, ContinuationToken: "token"}
	respWithoutToken := &params.ListRelationshipTuplesResponse{
		Tuples:            []params.RelationshipTuple{tuple},
		ContinuationToken: "",
	}
	s.mockJaasClient.EXPECT().ListRelationshipTuples(
		reqWithToken,
	).Return(respWithoutToken, nil).Times(1)

	client := s.getJaasClient()
	relations, err := client.ReadRelations(&tuple)
	s.Require().NoError(err)
	s.Require().Len(relations, 2)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestJaasSuite(t *testing.T) {
	suite.Run(t, new(JaasSuite))
}
