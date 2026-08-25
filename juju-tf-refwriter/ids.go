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

// entityID is the parsed components of a Juju resource identity.
type entityID struct {
	kind      resourceKind
	modelUUID string
	parts     []string
}

// identityShape describes how a resource kind's import identity decomposes
// into colon-separated parts. Zero-length partCount means the identity is
// opaque (model UUID / offer URL - the whole string is treated as the model
// UUID).
type identityShape struct {
	partCount int
	prefix    string
}

// shapeFor returns the expected identity format for a given resource kind.
// Returns nil for kinds whose identities are opaque (model UUID, offer URL).
func shapeFor(kind resourceKind) *identityShape {
	switch kind {
	case kindModel, kindOffer:
		return nil
	case kindApplication, kindSecret, kindSpace, kindStoragePool,
		kindAccessModel, kindAccessSecret:
		return &identityShape{partCount: 2}
	case kindMachine:
		return &identityShape{partCount: 3}
	case kindSSHKey:
		return &identityShape{partCount: 2, prefix: "sshkey:"}
	case kindIntegration:
		return &identityShape{partCount: 5}
	default:
		return nil
	}
}

// parseIdentity parses a Juju resource identity string for the given kind.
// Returns an error if the identity is malformed.
func parseIdentity(kind resourceKind, id string) (entityID, error) {
	e := entityID{kind: kind}
	shape := shapeFor(kind)
	if shape == nil {
		e.modelUUID = id
		return e, nil
	}
	s := id
	if shape.prefix != "" {
		if !strings.HasPrefix(s, shape.prefix) {
			return e, fmt.Errorf("malformed %s identity %q: expected prefix %q", kind, id, shape.prefix)
		}
		s = strings.TrimPrefix(s, shape.prefix)
	}
	parts := strings.Split(s, ":")
	if len(parts) != shape.partCount {
		return e, fmt.Errorf("malformed %s identity %q: expected %d colon-separated parts", kind, id, shape.partCount)
	}
	for _, p := range parts {
		if p == "" {
			return e, fmt.Errorf("malformed %s identity %q: empty component", kind, id)
		}
	}
	e.modelUUID, e.parts = parts[0], parts[1:]
	return e, nil
}

// part returns the i-th sub-component (app name for application, machine id
// for machine, secret/pool name for secret/storage_pool, app names for
// integration, etc.), or "" if out of range.
func (e entityID) part(i int) string {
	if len(e.parts) <= i {
		return ""
	}
	return e.parts[i]
}
