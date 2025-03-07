// Copyright 2023 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package juju

import (
	"fmt"
	"strings"

	jujuerrors "github.com/juju/errors"
)

func typedError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case strings.Contains(err.Error(), "not found"):
		return jujuerrors.WithType(err, jujuerrors.NotFound)
	case strings.Contains(err.Error(), "already exists"):
		return jujuerrors.WithType(err, jujuerrors.AlreadyExists)
	case strings.Contains(err.Error(), "user not valid"):
		return jujuerrors.WithType(err, jujuerrors.UserNotFound)
	case strings.Contains(err.Error(), "not valid"):
		return jujuerrors.WithType(err, jujuerrors.NotValid)
	case strings.Contains(err.Error(), "not implemented"):
		return jujuerrors.WithType(err, jujuerrors.NotImplemented)
	case strings.Contains(err.Error(), "not yet available"):
		return jujuerrors.WithType(err, jujuerrors.NotYetAvailable)
	default:
		return err
	}
}

// KeepWaitingForError is returned when we need to keep
// waiting for a resource to reach the desired state.
var KeepWaitingForError = &keepWaitingError{}

// keepWaitingError is returned when we need to keep waiting
// for an item to reach the desired state.
type keepWaitingError struct {
	item     string
	state    string
	endState string
}

// Error implements the error interface.
func (e *keepWaitingError) Error() string {
	return fmt.Sprintf("%q in state %q, waiting for %q", e.item, e.state, e.endState)
}
