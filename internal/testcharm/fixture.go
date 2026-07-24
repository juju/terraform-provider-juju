// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Package testcharm provides test helpers for packing the fixture charm
// directories under testdata/ into .charm archives. It has no dependencies
// on internal/juju or internal/testing, so it can be imported from both.
package testcharm

import (
	"archive/zip"
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	corebase "github.com/juju/juju/core/base"
	"github.com/juju/juju/domain/deployment/charm"
)

//go:embed testdata
var testdataFS embed.FS

// ZipFixture packs the fixture directory testdata/<fixture> into a .charm
// archive at <dir>/<fixture>.charm and returns the archive path. It creates
// dir if needed.
//
// Available fixtures under testdata/:
//   - test-charm-v1, test-charm-v2: named "test-charm", declaring
//     ubuntu@22.04 and ubuntu@24.04. Identical except for their content
//     file, so the two archives have distinct SHA-256 hashes.
//   - juju-qa-test: named "juju-qa-test", declaring ubuntu@22.04, for
//     tests switching between Charmhub and a local charm of the same name.
func ZipFixture(t *testing.T, fixture, dir string) string {
	t.Helper()

	archivePath, err := writeFixture(fixture, dir)
	if err != nil {
		t.Fatalf("packing test charm fixture %q: %v", fixture, err)
	}
	return archivePath
}

// writeFixture packs the fixture directory testdata/<fixture> into a .charm
// archive at <dir>/<fixture>.charm and returns the archive path. It creates
// dir if needed.
func writeFixture(fixture, dir string) (string, error) {
	root := filepath.Join("testdata", fixture)
	if _, err := fs.Stat(testdataFS, root); err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	archivePath := filepath.Join(dir, fixture+".charm")
	f, err := os.Create(archivePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	w := zip.NewWriter(f)
	err = fs.WalkDir(testdataFS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		body, err := testdataFS.ReadFile(path)
		if err != nil {
			return err
		}
		fw, err := w.Create(rel)
		if err != nil {
			return err
		}
		_, err = fw.Write(body)
		return err
	})
	if err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return archivePath, nil
}

// FixtureBases returns the bases declared in the fixture's manifest.yaml, in
// declaration order. An empty slice means the fixture declares no bases.
func FixtureBases(fixture string) ([]corebase.Base, error) {
	dir, err := os.MkdirTemp("", "testcharm")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	archivePath, err := writeFixture(fixture, dir)
	if err != nil {
		return nil, err
	}

	charmArchive, err := charm.ReadCharmArchive(archivePath)
	if err != nil {
		return nil, err
	}
	manifest := charmArchive.Manifest()
	if manifest == nil || len(manifest.Bases) == 0 {
		return nil, nil
	}
	return corebase.ParseManifestBases(manifest.Bases)
}
