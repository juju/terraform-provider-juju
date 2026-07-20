// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Package testcharm provides test helpers for building local charm archives.
// It has no dependencies on internal/juju or internal/testing, so it can be
// imported from both.
package testcharm

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BuildLocalCharm creates a minimal .charm archive at
// <dir>/<name>.charm that declares the given Ubuntu channels (e.g. "22.04",
// "24.04") in both metadata.yaml and manifest.yaml. The variable-content
// string ensures different calls with different content produce distinct
// SHA-256 hashes. It returns the path to the created archive.
func BuildLocalCharm(t *testing.T, dir, charmName, content string, baseChannels ...string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}

	var metaBases, manifestBases string
	for _, ch := range baseChannels {
		metaBases += fmt.Sprintf("  - name: ubuntu\n    channel: %q\n", ch)
		manifestBases += fmt.Sprintf("  - name: ubuntu\n    channel: %q\n    architectures:\n      - amd64\n", ch)
	}

	archivePath := filepath.Join(dir, charmName+".charm")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("creating charm archive: %v", err)
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	files := map[string]string{
		// metadata.yaml: v2 format with a bases stanza so Juju knows which
		// operating systems the charm supports.
		"metadata.yaml": fmt.Sprintf(
			"name: %s\nsummary: test charm\ndescription: acceptance test charm\nbases:\n%s",
			charmName, metaBases,
		),
		// manifest.yaml: must list the supported bases so the controller
		// accepts the charm. An empty list causes "charm does not define any
		// bases".
		"manifest.yaml": "bases:\n" + manifestBases,
		// dispatch satisfies AddLocalCharm's hasHooksOrDispatch requirement.
		"dispatch": "#!/bin/sh\n",
		// content is the only thing that varies between builds.
		"content": content,
	}
	for name, body := range files {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("adding %q to charm archive: %v", name, err)
		}
		if _, err = fw.Write([]byte(body)); err != nil {
			t.Fatalf("writing %q to charm archive: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing charm archive: %v", err)
	}
	return archivePath
}
