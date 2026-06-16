// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testing

import (
	"context"
	"time"

	apiclient "github.com/juju/juju/api/client/client"

	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

// WaitForRelationsJoined polls the Juju API until all relations in the given
// model have reached "joined" status, or the context times out.
func WaitForRelationsJoined(ctx context.Context, sc internaljuju.SharedClient, modelUUID string) error {
	conn, err := sc.GetConnection(ctx, &modelUUID)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	client := apiclient.NewClient(conn, sc.JujuLogger())
	_, err = wait.WaitFor(wait.WaitForCfg[struct{}, struct{}]{
		Context: ctx,
		GetData: func(ctx context.Context, _ struct{}) (struct{}, error) {
			status, err := client.Status(ctx, &apiclient.StatusArgs{})
			if err != nil {
				return struct{}{}, err
			}

			if len(status.Relations) == 0 {
				return struct{}{}, internaljuju.NewRetryReadErrorf("no relations found in model %s", modelUUID)
			}
			for _, rel := range status.Relations {
				if rel.Status.Status != "joined" {
					return struct{}{}, internaljuju.NewRetryReadErrorf(
						"relation %d not joined yet: %s",
						rel.Id,
						rel.Status.Status,
					)
				}
			}
			return struct{}{}, nil
		},
		Input:          struct{}{},
		NonFatalErrors: []error{internaljuju.RetryReadError},
		RetryConf: &wait.RetryConf{
			Delay:    1 * time.Second,
			MaxDelay: 5 * time.Second,
		},
	})
	return err
}
