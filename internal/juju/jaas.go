// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"errors"

	"github.com/canonical/jimm-go-sdk/v3/api"
	"github.com/canonical/jimm-go-sdk/v3/api/params"
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

func toJaasTuples(tuples []params.RelationshipTuple) []JaasTuple {
	out := make([]JaasTuple, 0, len(tuples))
	for _, tuple := range tuples {
		out = append(out, toJaasTuple(tuple))
	}
	return out
}

func toJaasTuple(tuple params.RelationshipTuple) JaasTuple {
	return JaasTuple{
		Object:   tuple.Object,
		Relation: tuple.Relation,
		Target:   tuple.TargetObject,
	}
}

// AddRelations attempts to create the provided slice of relationship tuples.
// An empty slice of tuples will return an error.
func (jc *jaasClient) AddRelations(tuples []JaasTuple) error {
	if len(tuples) == 0 {
		return errors.New("empty slice of tuples")
	}
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
// An empty slice of tuples will return an error.
func (jc *jaasClient) DeleteRelations(tuples []JaasTuple) error {
	if len(tuples) == 0 {
		return errors.New("empty slice of tuples")
	}
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
// An nil tuple pointer is invalid and will return an error.
func (jc *jaasClient) ReadRelations(ctx context.Context, tuple *JaasTuple) ([]JaasTuple, error) {
	if tuple == nil {
		return nil, errors.New("read relation tuple is nil")
	}

	conn, err := jc.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	relations := make([]JaasTuple, 0)
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
		relations = append(relations, toJaasTuples(resp.Tuples)...)
		if resp.ContinuationToken == "" {
			return relations, nil
		}
		req.ContinuationToken = resp.ContinuationToken
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}
}
