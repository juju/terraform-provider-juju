// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
	"github.com/juju/errors"
	jujuapi "github.com/juju/juju/api"
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

// JaasTuple represents a tuple object of used by JAAS for permissions management.
type JaasTuple struct {
	// Object represents the source side of the relation.
	Object string
	// Relation represents the level of access
	Relation string
	// Target represents the resource that you want `object` to have access to.
	Target string
}

func toAPITuples(tuples []JaasTuple) []params.RelationshipTuple {
	out := make([]params.RelationshipTuple, 0, len(tuples))
	for _, tuple := range tuples {
		out = append(out, toAPITuple(tuple))
	}
	return out
}

func toAPITuple(tuple JaasTuple) params.RelationshipTuple {
	return params.RelationshipTuple{
		Object:       tuple.Object,
		Relation:     tuple.Relation,
		TargetObject: tuple.Target,
	}
}

// AddRelations attempts to create the provided slice of relationship tuples.
// The caller is expected to populate the slice so that `len(tuples) > 0`.
func (jc *jaasClient) AddRelations(tuples []JaasTuple) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	cl := jc.getJaasApiClient(conn)
	req := params.AddRelationRequest{
		Tuples: toAPITuples(tuples),
	}
	return cl.AddRelation(&req)
}

// DeleteRelations attempts to delete the provided slice of relationship tuples.
// The caller is expected to populate the slice so that `len(tuples) > 0`.
func (jc *jaasClient) DeleteRelations(tuples []JaasTuple) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	cl := jc.getJaasApiClient(conn)
	req := params.RemoveRelationRequest{
		Tuples: toAPITuples(tuples),
	}
	return cl.RemoveRelation(&req)
}

// ReadRelations attempts to read relations that match the criteria defined by `tuple`.
// The caller is expected to provide a non-nil tuple.
func (jc *jaasClient) ReadRelations(tuple *JaasTuple) ([]params.RelationshipTuple, error) {
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
	req := &params.ListRelationshipTuplesRequest{Tuple: toAPITuple(*tuple)}
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
