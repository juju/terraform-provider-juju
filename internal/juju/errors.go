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

var ApplicationNotFoundError = &applicationNotFoundError{}

// ApplicationNotFoundError
type applicationNotFoundError struct {
	appName string
}

func (ae *applicationNotFoundError) Error() string {
	return fmt.Sprintf("application %q not found", ae.appName)
}

var StorageNotFoundError = &storageNotFoundError{}

// StorageNotFoundError
type storageNotFoundError struct {
	storageName string
}

func (se *storageNotFoundError) Error() string {
	return fmt.Sprintf("storage %q not found", se.storageName)
}

var KeepWaitingForDestroyError = &keepWaitingForDestroyError{}

// keepWaitingForDestroyError
type keepWaitingForDestroyError struct {
	itemDestroying string
	life           string
}

func (e *keepWaitingForDestroyError) Error() string {

	return fmt.Sprintf("%q still alive, life = %s", e.itemDestroying, e.life)
}

var NoIntegrationFoundError = &noIntegrationFoundError{}

// NoIntegrationFoundError
type noIntegrationFoundError struct {
	ModelUUID string
}

func (ie *noIntegrationFoundError) Error() string {
	return fmt.Sprintf("no integrations exist in model %v", ie.ModelUUID)
}

var ModelNotFoundError = &modelNotFoundError{}

type modelNotFoundError struct {
	uuid string
	name string
}

func (me *modelNotFoundError) Error() string {
	toReturn := "model %q was not found"
	if me.name != "" {
		return fmt.Sprintf(toReturn, me.name)
	}
	return fmt.Sprintf(toReturn, me.uuid)
}

var SecretNotFoundError = &secretNotFoundError{}

type secretNotFoundError struct {
	secretId string
}

func (se *secretNotFoundError) Error() string {
	if se.secretId != "" {
		return fmt.Sprintf("secret %q was not found", se.secretId)
	} else {
		return "secret was not found"
	}
}
