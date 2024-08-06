// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
	jujuapi "github.com/juju/juju/api"

	"github.com/juju/errors"
)

type jaasClient struct {
	SharedClient
	getJaasApiClient func(jujuapi.Connection) JaasAPIClient
}

func newJaasClient(sc SharedClient) *jaasClient {
	return &jaasClient{
		SharedClient: sc,
		getJaasApiClient: func(conn jujuapi.Connection) JaasAPIClient {
			return api.NewClient(conn)
		},
	}
}

// AddRelations attempts to create the provided slice of relationship tuples.
func (jc *jaasClient) AddRelations(tuples []params.RelationshipTuple) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	cl := jc.getJaasApiClient(conn)
	req := params.AddRelationRequest{
		Tuples: tuples,
	}
	return cl.AddRelation(&req)
}

// DeleteRelations attempts to delete the provided slice of relationship tuples.
func (jc *jaasClient) DeleteRelations(tuples []params.RelationshipTuple) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	cl := jc.getJaasApiClient(conn)
	req := params.RemoveRelationRequest{
		Tuples: tuples,
	}
	return cl.RemoveRelation(&req)
}

// ReadRelations attempts to read relations that match the criteria defined by `tuple`.
func (jc *jaasClient) ReadRelations(tuple *params.RelationshipTuple) ([]params.RelationshipTuple, error) {
	if tuple == nil {
		return nil, errors.New("add relation request nil")
	}

	conn, err := jc.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	relations := make([]params.RelationshipTuple, 0)
	req := &params.ListRelationshipTuplesRequest{Tuple: *tuple}
	for {
		resp, err := client.ListRelationshipTuples(req)
		if err != nil {
			jc.Errorf(err, "call to ListRelationshipTuples failed")
			return nil, err
		}
		if len(resp.Errors) > 0 {
			jc.Errorf(err, "call to ListRelationshipTuples contained error(s)")
			return nil, errors.New(resp.Errors[0])
		}
		relations = append(relations, resp.Tuples...)
		if resp.ContinuationToken == "" {
			return relations, nil
		}
		req.ContinuationToken = resp.ContinuationToken
	}
}
