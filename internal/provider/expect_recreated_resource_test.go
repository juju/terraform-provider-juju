// Copyright 2024 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package provider

import (
	"context"
	"errors"
	"fmt"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

var _ plancheck.PlanCheck = expectRecreatedResource{}

type expectRecreatedResource struct {
	resourceName string
}

// CheckPlan implements the plan check logic.
func (e expectRecreatedResource) CheckPlan(ctx context.Context, req plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	var result []error

	for _, rc := range req.Plan.ResourceChanges {
		if rc.Address == e.resourceName {
			changes := rc.Change.Actions
			if len(changes) != 2 {
				result = append(result, fmt.Errorf("2 changes for resource %s expected (delete and create): %d found", rc.Address, len(changes)))
				continue
			}
			if changes[0] != tfjson.ActionDelete && changes[1] != tfjson.ActionCreate {
				result = append(result, fmt.Errorf("expected delete then create for resource %s, but found planned action(s): %v", rc.Address, rc.Change.Actions))
			}
		}
	}

	resp.Error = errors.Join(result...)
}

// expectRecreatedResource returns a plan check that asserts is a delete and create change present.
// All output and resource changes found will be aggregated and returned in a plan check error.
func ExpectRecreatedResource(resourceName string) plancheck.PlanCheck {
	return expectRecreatedResource{
		resourceName: resourceName,
	}
}
