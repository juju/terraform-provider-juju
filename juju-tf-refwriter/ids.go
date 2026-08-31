// Copyright 2026 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.
package main

import (
	"fmt"
	"strings"
)

// resourceKind classifies a Juju resource by its Terraform type.
type resourceKind string

const (
	kindModel        resourceKind = "model"
	kindApplication  resourceKind = "application"
	kindMachine      resourceKind = "machine"
	kindSecret       resourceKind = "secret"
	kindSpace        resourceKind = "space"
	kindSSHKey       resourceKind = "ssh_key"
	kindStoragePool  resourceKind = "storage_pool"
	kindOffer        resourceKind = "offer"
	kindIntegration  resourceKind = "integration"
	kindAccessModel  resourceKind = "access_model"
	kindAccessSecret resourceKind = "access_secret"
	kindUnknown      resourceKind = "unknown"
)

// kindOf maps a Terraform resource type (e.g. "juju_application") to a
// resourceKind. Unknown types return kindUnknown.
func kindOf(resourceType string) resourceKind {
	switch resourceType {
	case "juju_model":
		return kindModel
	case "juju_application":
		return kindApplication
	case "juju_machine":
		return kindMachine
	case "juju_secret":
		return kindSecret
	case "juju_space":
		return kindSpace
	case "juju_ssh_key":
		return kindSSHKey
	case "juju_storage_pool":
		return kindStoragePool
	case "juju_offer":
		return kindOffer
	case "juju_integration":
		return kindIntegration
	case "juju_access_model":
		return kindAccessModel
	case "juju_access_secret":
		return kindAccessSecret
	default:
		return kindUnknown
	}
}

// entityID holds the model UUID and, where relevant, the name parsed from a
// Juju resource import identity ("<modelUUID>:<name>..."). Model and offer
// identities are opaque, so the whole string is the model UUID.
type entityID struct {
	kind      resourceKind
	modelUUID string
	name      string
}

// parseIdentity splits a Juju resource identity into its model UUID and
// name. Model/offer identities are opaque and stored whole as the UUID.
func parseIdentity(kind resourceKind, id string) (entityID, error) {
	e := entityID{kind: kind}
	if kind == kindModel || kind == kindOffer {
		e.modelUUID = id
		return e, nil
	}
	modelUUID, rest, ok := strings.Cut(id, ":")
	if !ok || modelUUID == "" {
		return e, fmt.Errorf("malformed %s identity %q: expected \"<modelUUID>:...\"", kind, id)
	}
	e.modelUUID = modelUUID
	e.name, _, _ = strings.Cut(rest, ":")
	return e, nil
}
