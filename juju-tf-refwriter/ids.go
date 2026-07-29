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

// entityID holds the parsed components of a Juju resource identity.
//
// The identity formats mirror the provider's import ID formats:
//
//   - model:          <model-uuid>
//   - application:    <model-uuid>:<app-name>
//   - machine:        <model-uuid>:<machine-id>:<machine-name>
//   - secret:         <model-uuid>:<secret-name>
//   - space:          <model-uuid>:<space-name>
//   - ssh_key:        sshkey:<model-uuid>:<key-identifier>
//   - storage_pool:   <model-uuid>:<pool-name>
//   - offer:          <offer-url>   (e.g. controller:model.app)
//   - integration:    <model-uuid>:<app1>:<ep1>:<app2>:<ep2>
type entityID struct {
	kind        resourceKind
	modelUUID   string
	appName     string
	machineID   string
	machineName string
	secretName  string
	spaceName   string
	keyID       string
	poolName    string
	offerURL    string
	// For integrations: the two app:endpoint pairs.
	app1, ep1 string
	app2, ep2 string
}

// parseIdentity parses a Juju resource identity string for the given kind.
// It returns an error if the identity is malformed for that kind.
func parseIdentity(kind resourceKind, id string) (entityID, error) {
	e := entityID{kind: kind}
	switch kind {
	case kindModel:
		// A model UUID. We don't validate the UUID shape strictly; the
		// provider does that. We only require it to be non-empty and to
		// contain no colon (so it can't be confused with composite IDs).
		if id == "" || strings.Contains(id, ":") {
			return e, fmt.Errorf("malformed model identity %q: expected a bare model UUID", id)
		}
		e.modelUUID = id
	case kindApplication:
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return e, fmt.Errorf("malformed application identity %q: expected <model-uuid>:<app-name>", id)
		}
		e.modelUUID, e.appName = parts[0], parts[1]
	case kindMachine:
		parts := strings.SplitN(id, ":", 3)
		if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
			return e, fmt.Errorf("malformed machine identity %q: expected <model-uuid>:<machine-id>:<machine-name>", id)
		}
		e.modelUUID, e.machineID, e.machineName = parts[0], parts[1], parts[2]
	case kindSecret:
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return e, fmt.Errorf("malformed secret identity %q: expected <model-uuid>:<secret-name>", id)
		}
		e.modelUUID, e.secretName = parts[0], parts[1]
	case kindSpace:
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return e, fmt.Errorf("malformed space identity %q: expected <model-uuid>:<space-name>", id)
		}
		e.modelUUID, e.spaceName = parts[0], parts[1]
	case kindSSHKey:
		// sshkey:<model-uuid>:<key-identifier>
		const prefix = "sshkey:"
		if !strings.HasPrefix(id, prefix) {
			return e, fmt.Errorf("malformed ssh_key identity %q: expected sshkey:<model-uuid>:<key-identifier>", id)
		}
		rest := strings.TrimPrefix(id, prefix)
		parts := strings.SplitN(rest, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return e, fmt.Errorf("malformed ssh_key identity %q: expected sshkey:<model-uuid>:<key-identifier>", id)
		}
		e.modelUUID, e.keyID = parts[0], parts[1]
	case kindStoragePool:
		parts := strings.SplitN(id, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return e, fmt.Errorf("malformed storage_pool identity %q: expected <model-uuid>:<pool-name>", id)
		}
		e.modelUUID, e.poolName = parts[0], parts[1]
	case kindOffer:
		// Offer URLs don't contain a model UUID in the same form; they're
		// of the form [controller:]model.app. We keep the whole URL.
		if id == "" {
			return e, fmt.Errorf("malformed offer identity %q: expected an offer URL", id)
		}
		e.offerURL = id
	case kindIntegration:
		// <model-uuid>:<app1>:<ep1>:<app2>:<ep2>
		parts := strings.Split(id, ":")
		if len(parts) != 5 {
			return e, fmt.Errorf("malformed integration identity %q: expected <model-uuid>:<app1>:<ep1>:<app2>:<ep2>", id)
		}
		for _, p := range parts {
			if p == "" {
				return e, fmt.Errorf("malformed integration identity %q: empty component", id)
			}
		}
		e.modelUUID, e.app1, e.ep1, e.app2, e.ep2 = parts[0], parts[1], parts[2], parts[3], parts[4]
	case kindAccessModel, kindAccessSecret:
		// Access resources are addressed by <model-uuid>:<name> (the name
		// being the secret name for access_secret; access_model uses just
		// the model UUID as its identity target). We treat both as
		// model-scoped for the purpose of rewriting model_uuid.
		parts := strings.SplitN(id, ":", 2)
		if len(parts) < 1 || parts[0] == "" {
			return e, fmt.Errorf("malformed %s identity %q", kind, id)
		}
		e.modelUUID = parts[0]
		if len(parts) == 2 {
			e.secretName = parts[1]
		}
	default:
		return e, fmt.Errorf("unknown resource kind %q", kind)
	}
	return e, nil
}
