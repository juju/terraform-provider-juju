// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"path/filepath"
	"testing"

	corebase "github.com/juju/juju/core/base"
	coreversion "github.com/juju/juju/core/version"
	"github.com/juju/juju/environs/config"
	"github.com/juju/utils/v4"
	"github.com/stretchr/testify/require"

	testcharm "github.com/juju/terraform-provider-juju/internal/testcharm"
)

// buildLocalCharm delegates to testcharm.BuildLocalCharm so test bodies
// in this file read naturally.
func buildLocalCharm(t *testing.T, dir, charmName, content string, baseChannels ...string) string {
	t.Helper()
	return testcharm.BuildLocalCharm(t, dir, charmName, content, baseChannels...)
}

// minModelConfig returns a minimal model config attribute map suitable for
// config.New.  callers may add or override keys before passing to
// MockModelConfigAPIClient.
func minModelConfig(t *testing.T, extra map[string]interface{}) map[string]interface{} {
	t.Helper()
	base := map[string]interface{}{
		"name":            "test",
		"type":            "manual",
		"uuid":            utils.MustNewUUID().String(),
		"controller-uuid": utils.MustNewUUID().String(),
		"firewall-mode":   "instance",
		"secret-backend":  "auto",
		"image-stream":    "testing",
	}
	for k, v := range extra {
		base[k] = v
	}
	return base
}

// ---- ReadLocalCharmInfo ----

func TestReadLocalCharmInfo_ReturnsNameHashAndBases(t *testing.T) {
	dir := t.TempDir()
	path := buildLocalCharm(t, dir, "my-charm", "v1-content", "22.04", "24.04")

	info, err := ReadLocalCharmInfo(path)
	require.NoError(t, err)

	require.Equal(t, "my-charm", info.Name)
	require.Len(t, info.Hash, 64, "hash should be a 64-char hex SHA-256")
	require.Len(t, info.SupportedBases, 2)
}

func TestReadLocalCharmInfo_HashChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	p1 := buildLocalCharm(t, filepath.Join(dir, "v1"), "charm", "content-a", "22.04")
	p2 := buildLocalCharm(t, filepath.Join(dir, "v2"), "charm", "content-b", "22.04")

	i1, err := ReadLocalCharmInfo(p1)
	require.NoError(t, err)
	i2, err := ReadLocalCharmInfo(p2)
	require.NoError(t, err)

	require.NotEqual(t, i1.Hash, i2.Hash, "different content must produce different hashes")
}

func TestReadLocalCharmInfo_MissingFile(t *testing.T) {
	_, err := ReadLocalCharmInfo("/nonexistent/path/charm.charm")
	require.Error(t, err)
}

// ---- CheckLocalCharmBase ----

func TestCheckLocalCharmBase_Compatible(t *testing.T) {
	dir := t.TempDir()
	path := buildLocalCharm(t, dir, "c", "v1", "22.04")
	info, err := ReadLocalCharmInfo(path)
	require.NoError(t, err)

	require.NoError(t, CheckLocalCharmBase(info, "ubuntu@22.04"))
}

func TestCheckLocalCharmBase_Incompatible(t *testing.T) {
	dir := t.TempDir()
	path := buildLocalCharm(t, dir, "c", "v1", "22.04")
	info, err := ReadLocalCharmInfo(path)
	require.NoError(t, err)

	err = CheckLocalCharmBase(info, "ubuntu@24.04")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ubuntu@24.04")
	require.Contains(t, err.Error(), "ubuntu@22.04")
}

func TestCheckLocalCharmBase_NoManifestBases_AlwaysOK(t *testing.T) {
	// When the charm declares no bases (old-style), any base is accepted.
	// We use a hand-crafted LocalCharmInfo because the archive reader rejects
	// a manifest.yaml with no bases list; the code path we exercise here is
	// CheckLocalCharmBase's early-return for len(SupportedBases)==0.
	info := LocalCharmInfo{
		Name:           "old-charm",
		Hash:           "abc123",
		SupportedBases: nil,
	}
	require.NoError(t, CheckLocalCharmBase(info, "ubuntu@24.04"))
	require.NoError(t, CheckLocalCharmBase(info, "ubuntu@22.04"))
}

func TestCheckLocalCharmBase_MultiBase_CompatibleMatch(t *testing.T) {
	dir := t.TempDir()
	path := buildLocalCharm(t, dir, "c", "v1", "22.04", "24.04")
	info, err := ReadLocalCharmInfo(path)
	require.NoError(t, err)

	require.NoError(t, CheckLocalCharmBase(info, "ubuntu@22.04"))
	require.NoError(t, CheckLocalCharmBase(info, "ubuntu@24.04"))
}

// ---- selectLocalCharmBase ----

// supportedBases parses a slice of "os@channel" strings into []corebase.Base.
func supportedBases(t *testing.T, bases ...string) []corebase.Base {
	t.Helper()
	out := make([]corebase.Base, 0, len(bases))
	for _, b := range bases {
		parsed, err := corebase.ParseBaseFromString(b)
		require.NoError(t, err, "parsing base %q", b)
		out = append(out, parsed)
	}
	return out
}

// newModelConfig builds a *config.Config from the minimal model config
// attributes plus any extras (e.g. "default-base"). selectLocalCharmBase is a
// pure function of this config, so the tests need no API mocks.
func newModelConfig(t *testing.T, extra map[string]interface{}) *config.Config {
	t.Helper()
	attrs := minModelConfig(t, extra)
	cfg, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)
	return cfg
}

// TestSelectLocalCharmBase_Step1_UserSupplied verifies that when the user
// provides an explicit base that is in the charm's supported bases list,
// it is returned immediately without consulting the model config.
func TestSelectLocalCharmBase_Step1_UserSupplied(t *testing.T) {
	// The model config sets an incompatible default-base to prove the
	// user-supplied base takes precedence over the model-default fallback.
	cfg := newModelConfig(t, map[string]interface{}{
		"default-base":  "ubuntu@20.04",
		"agent-version": "4.0.0",
	})

	requested, _ := corebase.ParseBaseFromString("ubuntu@22.04")
	got, err := selectLocalCharmBase(cfg, requested, supportedBases(t, "ubuntu@22.04", "ubuntu@24.04"))
	require.NoError(t, err)
	require.Equal(t, "22.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_Step1_UserSupplied_Incompatible verifies that
// selectLocalCharmBase returns an error when the user-supplied base is not
// in the charm's manifest. Note: the acceptance test TestAcc_*_BaseMismatch
// exercises the same incompatibility but via ValidateConfig (plan-time),
// which calls CheckLocalCharmBase rather than selectLocalCharmBase.
func TestSelectLocalCharmBase_Step1_UserSupplied_Incompatible(t *testing.T) {
	cfg := newModelConfig(t, map[string]interface{}{"agent-version": "4.0.0"})

	requested, _ := corebase.ParseBaseFromString("ubuntu@20.04")
	_, err := selectLocalCharmBase(cfg, requested, supportedBases(t, "ubuntu@22.04"))
	require.Error(t, err)
}

// TestSelectLocalCharmBase_Step2_ModelDefault verifies that when no base is
// requested but the model has a default-base that is compatible with the
// charm's manifest, that base is returned.
func TestSelectLocalCharmBase_Step2_ModelDefault(t *testing.T) {
	cfg := newModelConfig(t, map[string]interface{}{
		"default-base":  "ubuntu@22.04",
		"agent-version": "4.0.0",
	})

	// Charm only supports 22.04; LTS default (24.04) would be incompatible,
	// so step 2 wins here.
	got, err := selectLocalCharmBase(cfg, corebase.Base{}, supportedBases(t, "ubuntu@22.04"))
	require.NoError(t, err)
	require.Equal(t, "22.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_Step2_ModelDefault_Incompatible_FallsThrough
// verifies that when the model default-base is set but incompatible with the
// charm, the selector falls through to step 3 (LTS default).
func TestSelectLocalCharmBase_Step2_ModelDefault_Incompatible_FallsThrough(t *testing.T) {
	// Model default is 20.04, but the charm only supports the LTS base.
	cfg := newModelConfig(t, map[string]interface{}{
		"default-base":  "ubuntu@20.04",
		"agent-version": "4.0.0",
	})

	lts := coreversion.DefaultSupportedLTSBase()
	got, err := selectLocalCharmBase(cfg, corebase.Base{}, supportedBases(t, lts.OS+"@"+lts.Channel.Track))
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track,
		"should fall through to LTS default when model default is incompatible")
}

// TestSelectLocalCharmBase_Step3_LTSDefault verifies that when no base is
// requested and no model default is set, the Juju LTS default is used if
// compatible.
func TestSelectLocalCharmBase_Step3_LTSDefault(t *testing.T) {
	// No default-base in model config.
	cfg := newModelConfig(t, map[string]interface{}{"agent-version": "4.0.0"})

	lts := coreversion.DefaultSupportedLTSBase()
	got, err := selectLocalCharmBase(cfg, corebase.Base{}, supportedBases(t, lts.OS+"@"+lts.Channel.Track))
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track)
}

// TestSelectLocalCharmBase_Step4_FirstManifestBase verifies that when no
// base is requested, the model has no default, and the LTS default is not
// compatible, the first base declared in the charm's manifest is selected.
func TestSelectLocalCharmBase_Step4_FirstManifestBase(t *testing.T) {
	cfg := newModelConfig(t, map[string]interface{}{"agent-version": "4.0.0"})

	// Only ubuntu@20.04 is supported — neither the model default nor the LTS
	// default (ubuntu@24.04) matches, so we land at step 4.
	got, err := selectLocalCharmBase(cfg, corebase.Base{}, supportedBases(t, "ubuntu@20.04"))
	require.NoError(t, err)
	require.Equal(t, "20.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_OldStyleCharm_LTSFallback verifies that an
// old-style charm with no manifest bases and no user-supplied or model-default
// base receives the Juju LTS default. corecharm.BaseForCharm returns the
// requested base unchanged when supportedBases is empty, so step 3 (LTS)
// succeeds rather than reaching the MissingBaseError in step 4.
func TestSelectLocalCharmBase_OldStyleCharm_LTSFallback(t *testing.T) {
	cfg := newModelConfig(t, map[string]interface{}{"agent-version": "4.0.0"})

	// corecharm.BaseForCharm(lts, nil): supportedBases is empty and the
	// requested base (lts) is non-empty, so it returns (lts, nil) — the
	// old-style charm case. The selector therefore succeeds at step 3.
	lts := coreversion.DefaultSupportedLTSBase()
	got, err := selectLocalCharmBase(cfg, corebase.Base{}, nil)
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track,
		"old-style charm with no manifest bases should get the LTS default")
}
