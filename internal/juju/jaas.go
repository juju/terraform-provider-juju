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

// JaasGroup represents a JAAS group used for permissions management.
type JaasGroup struct {
	Name string
	UUID string
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

// AddGroup attempts to create a new group with the provided name.
func (jc *jaasClient) AddGroup(name string) (string, error) {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	req := params.AddGroupRequest{Name: name}

	resp, err := client.AddGroup(&req)
	if err != nil {
		return "", err
	}
	return resp.UUID, nil
}

// ReadGroupByUUID attempts to read a group that matches the provided UUID.
func (jc *jaasClient) ReadGroupByUUID(uuid string) (*JaasGroup, error) {
	return jc.readGroup(&params.GetGroupRequest{UUID: uuid})
}

// ReadGroupByName attempts to read a group that matches the provided name.
func (jc *jaasClient) ReadGroupByName(name string) (*JaasGroup, error) {
	return jc.readGroup(&params.GetGroupRequest{Name: name})
}

func (jc *jaasClient) readGroup(req *params.GetGroupRequest) (*JaasGroup, error) {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	resp, err := client.GetGroup(req)
	if err != nil {
		return nil, err
	}
	return &JaasGroup{Name: resp.Name, UUID: resp.UUID}, nil
}

// RenameGroup attempts to rename a group that matches the provided name.
func (jc *jaasClient) RenameGroup(name, newName string) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	req := params.RenameGroupRequest{Name: name, NewName: newName}
	return client.RenameGroup(&req)
}

// RemoveGroup attempts to remove a group that matches the provided name.
func (jc *jaasClient) RemoveGroup(name string) error {
	conn, err := jc.GetConnection(nil)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := jc.getJaasApiClient(conn)
	req := params.RemoveGroupRequest{Name: name}
	return client.RemoveGroup(&req)
}
