// Copyright 2025 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"github.com/juju/errors"
	"github.com/juju/juju/api"
	"github.com/juju/juju/api/client/annotations"
	"github.com/juju/names/v5"
)

type annotationsClient struct {
	SharedClient

	getAnnotationsAPIClient func(connection api.Connection) AnnotationsAPIClient
}

// SetAnnotationsInput contains the fields needed to set the annotations for an entity.
type SetAnnotationsInput struct {
	ModelName   string
	EntityTag   names.Tag
	Annotations map[string]string
}

// GetAnnotationsInput contains the fields needed to get the annotations for an entity.
type GetAnnotationsInput struct {
	ModelName string
	EntityTag names.Tag
}

// GetAnnotationsOutput contains the results of getting the annotation for an entity.
type GetAnnotationsOutput struct {
	EntityTag   names.Tag
	Annotations map[string]string
}

func newAnnotationsClient(sc SharedClient) *annotationsClient {
	return &annotationsClient{
		SharedClient: sc,
		getAnnotationsAPIClient: func(connection api.Connection) AnnotationsAPIClient {
			return annotations.NewClient(connection)
		},
	}
}

// SetAnnotations set the annotations for the entity specified.
// To unset a specific annotation a empty string "" needs to be set.
func (c *annotationsClient) SetAnnotations(input *SetAnnotationsInput) error {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	annotationsAPIClient := c.getAnnotationsAPIClient(conn)

	args := map[string]map[string]string{
		input.EntityTag.String(): input.Annotations,
	}

	results, err := annotationsAPIClient.Set(args)
	if err != nil {
		return err
	}
	// if there are no errors the results slice is empty.
	if len(results) > 0 {
		if len(results) != 1 {
			return errors.Errorf("should receive just a single error for %q", input.EntityTag)
		}
		if err := results[0].Error; err != nil {
			return err
		}
	}

	return nil
}

// GetAnnotations gets the annotation for an entity.
func (c *annotationsClient) GetAnnotations(input *GetAnnotationsInput) (*GetAnnotationsOutput, error) {
	conn, err := c.GetConnection(&input.ModelName)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()

	annotationsAPIClient := c.getAnnotationsAPIClient(conn)

	results, err := annotationsAPIClient.Get([]string{input.EntityTag.String()})
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, errors.NotFoundf("annotations for entity %q", input.EntityTag)
	}

	if len(results) > 1 {
		return nil, errors.Errorf("should receive just a single result for %q", input.EntityTag)
	}

	result := results[0]
	if err := result.Error.Error; err != nil {
		return nil, err
	}
	return &GetAnnotationsOutput{
		EntityTag:   input.EntityTag,
		Annotations: result.Annotations,
	}, nil
}
