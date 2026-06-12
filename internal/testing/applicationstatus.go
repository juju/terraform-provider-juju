// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package testing

import (
	"context"
	"fmt"
	"time"

	apiclient "github.com/juju/juju/api/client/client"
	"github.com/juju/juju/rpc/params"

	internaljuju "github.com/juju/terraform-provider-juju/internal/juju"
	"github.com/juju/terraform-provider-juju/internal/wait"
)

// WaitForApplicationIdle waits until an application is present in status,
// is not erroring, and all of its units are idle
func WaitForApplicationIdle(ctx context.Context, statusClient internaljuju.ClientAPIClient, appName string) error {
	_, err := wait.WaitFor(wait.WaitForCfg[struct{}, struct{}]{
		Context: ctx,
		GetData: func(ctx context.Context, _ struct{}) (struct{}, error) {
			status, err := statusClient.Status(ctx, &apiclient.StatusArgs{})
			if err != nil {
				return struct{}{}, err
			}

			appStatus, ok := status.Applications[appName]
			if !ok {
				return struct{}{}, internaljuju.NewRetryReadErrorf(
					"application %s not found in status output",
					appName,
				)
			}
			if appStatus.Status.Status == "error" {
				return struct{}{}, fmt.Errorf("application %s entered error status", appName)
			}
			if err := assertApplicationUnitsIdle(appName, appStatus.Units); err != nil {
				return struct{}{}, err
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

func assertApplicationUnitsIdle(appName string, units map[string]params.UnitStatus) error {
	if len(units) == 0 {
		return internaljuju.NewRetryReadErrorf(
			"application %s has no units in status output yet",
			appName,
		)
	}

	for unitName, unitStatus := range units {
		if unitStatus.AgentStatus.Status == "error" {
			return fmt.Errorf("unit %s agent entered error status", unitName)
		}
		if unitStatus.WorkloadStatus.Status == "error" {
			return fmt.Errorf("unit %s workload entered error status", unitName)
		}
		if unitStatus.AgentStatus.Status != "idle" {
			return internaljuju.NewRetryReadErrorf(
				"unit %s agent not idle yet: %s",
				unitName,
				unitStatus.AgentStatus.Status,
			)
		}
	}

	return nil
}
