// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testing

import (
	"context"
	"fmt"
	"time"

	apiclient "github.com/juju/juju/api/client/client"

	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
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

	for {
		status, err := client.Status(ctx, &apiclient.StatusArgs{})
		if err != nil {
			return err
		}

		if len(status.Relations) > 0 {
			allJoined := true
			for _, rel := range status.Relations {
				if rel.Status.Status != "joined" {
					allJoined = false
					break
				}
			}
			if allJoined {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for relations in model %s to be joined", modelUUID)
		case <-time.After(5 * time.Second):
		}
	}
}
