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

	root := filepath.Join("testdata", fixture)
	if _, err := fs.Stat(testdataFS, root); err != nil {
		t.Fatalf("unknown test charm fixture %q: %v", fixture, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %q: %v", dir, err)
	}

	archivePath := filepath.Join(dir, fixture+".charm")
	f, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("creating charm archive: %v", err)
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
		t.Fatalf("adding fixture %q files to charm archive: %v", fixture, err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing charm archive: %v", err)
	}
	return archivePath
}
