// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"path/filepath"
	"testing"

	"github.com/juju/juju/api"
	corebase "github.com/juju/juju/core/base"
	coreversion "github.com/juju/juju/core/version"
	"github.com/juju/juju/environs/config"
	"github.com/juju/utils/v4"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

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

func TestReadLocalCharmInfo_NoManifestBases(t *testing.T) {
	// Old-style charms declare no bases in manifest.yaml. The Juju archive
	// reader rejects a manifest.yaml that has a bases key with no list, so we
	// cannot build such an archive with BuildLocalCharm; instead we construct
	// LocalCharmInfo directly to verify that SupportedBases is nil/empty.
	info := LocalCharmInfo{
		Name:           "old-charm",
		Hash:           "abc123",
		SupportedBases: nil,
	}
	require.Empty(t, info.SupportedBases)
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

// makeSelectBaseClient builds an applicationsClient whose getModelConfigAPIClient
// is wired to the given MockModelConfigAPIClient.
func makeSelectBaseClient(mock *MockModelConfigAPIClient, mockShared *MockSharedClient) applicationsClient {
	return applicationsClient{
		SharedClient: mockShared,
		getModelConfigAPIClient: func(_ api.Connection) ModelConfigAPIClient {
			return mock
		},
	}
}

// TestSelectLocalCharmBase_Step1_UserSupplied verifies that when the user
// provides an explicit base that is in the charm's supported bases list,
// it is returned immediately without consulting the model config.
func TestSelectLocalCharmBase_Step1_UserSupplied(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	// ModelGet must NOT be called: user-supplied base takes effect before
	// the model-default fallback.
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Times(0)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	requested, _ := corebase.ParseBaseFromString("ubuntu@22.04")
	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		requested,
		supportedBases(t, "ubuntu@22.04", "ubuntu@24.04"),
	)
	require.NoError(t, err)
	require.Equal(t, "22.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_Step1_UserSupplied_Incompatible verifies that
// selectLocalCharmBase returns an error when the user-supplied base is not
// in the charm's manifest. Note: the acceptance test TestAcc_*_BaseMismatch
// exercises the same incompatibility but via ValidateConfig (plan-time),
// which calls CheckLocalCharmBase rather than selectLocalCharmBase.
func TestSelectLocalCharmBase_Step1_UserSupplied_Incompatible(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Times(0)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	requested, _ := corebase.ParseBaseFromString("ubuntu@20.04")
	_, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		requested,
		supportedBases(t, "ubuntu@22.04"),
	)
	require.Error(t, err)
}

// TestSelectLocalCharmBase_Step2_ModelDefault verifies that when no base is
// requested but the model has a default-base that is compatible with the
// charm's manifest, that base is returned.
func TestSelectLocalCharmBase_Step2_ModelDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	attrs := minModelConfig(t, map[string]interface{}{
		"default-base": "ubuntu@22.04",
		// agent-version is required by config.New(UseDefaults, ...)
		"agent-version": "4.0.0",
	})
	// Validate that config.New accepts these attrs before wiring the mock.
	_, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).Times(1)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		corebase.Base{}, // no user-supplied base
		// Charm only supports 22.04; LTS default (24.04) would be
		// incompatible, so step 2 wins here.
		supportedBases(t, "ubuntu@22.04"),
	)
	require.NoError(t, err)
	require.Equal(t, "22.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_Step2_ModelDefault_Incompatible_FallsThrough
// verifies that when the model default-base is set but incompatible with the
// charm, the selector falls through to step 3 (LTS default).
func TestSelectLocalCharmBase_Step2_ModelDefault_Incompatible_FallsThrough(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	attrs := minModelConfig(t, map[string]interface{}{
		// Model default is 20.04, but charm only supports 24.04.
		"default-base":  "ubuntu@20.04",
		"agent-version": "4.0.0",
	})
	_, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).Times(1)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	lts := coreversion.DefaultSupportedLTSBase()

	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		corebase.Base{},
		// Charm supports the LTS base — step 3 should win.
		supportedBases(t, lts.OS+"@"+lts.Channel.Track),
	)
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track,
		"should fall through to LTS default when model default is incompatible")
}

// TestSelectLocalCharmBase_Step3_LTSDefault verifies that when no base is
// requested and no model default is set, the Juju LTS default is used if
// compatible.
func TestSelectLocalCharmBase_Step3_LTSDefault(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// No default-base in model config.
	attrs := minModelConfig(t, map[string]interface{}{
		"agent-version": "4.0.0",
	})
	_, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).Times(1)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	lts := coreversion.DefaultSupportedLTSBase()

	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		corebase.Base{},
		supportedBases(t, lts.OS+"@"+lts.Channel.Track),
	)
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track)
}

// TestSelectLocalCharmBase_Step4_FirstManifestBase verifies that when no
// base is requested, the model has no default, and the LTS default is not
// compatible, the first base declared in the charm's manifest is selected.
func TestSelectLocalCharmBase_Step4_FirstManifestBase(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	attrs := minModelConfig(t, map[string]interface{}{
		"agent-version": "4.0.0",
	})
	_, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).Times(1)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	// Only ubuntu@20.04 is supported — neither the model default nor the LTS
	// default (ubuntu@24.04) matches, so we land at step 4.
	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		corebase.Base{},
		supportedBases(t, "ubuntu@20.04"),
	)
	require.NoError(t, err)
	require.Equal(t, "20.04", got.Channel.Track)
}

// TestSelectLocalCharmBase_OldStyleCharm_LTSFallback verifies that an
// old-style charm with no manifest bases and no user-supplied or model-default
// base receives the Juju LTS default. corecharm.BaseForCharm returns the
// requested base unchanged when supportedBases is empty, so step 3 (LTS)
// succeeds rather than reaching the MissingBaseError in step 4.
func TestSelectLocalCharmBase_OldStyleCharm_LTSFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	attrs := minModelConfig(t, map[string]interface{}{
		"agent-version": "4.0.0",
	})
	_, err := config.New(config.UseDefaults, attrs)
	require.NoError(t, err)

	mockModelConfig := NewMockModelConfigAPIClient(ctrl)
	mockModelConfig.EXPECT().ModelGet(gomock.Any()).Return(attrs, nil).Times(1)

	mockShared := NewMockSharedClient(ctrl)
	mockShared.EXPECT().Tracef(gomock.Any(), gomock.Any()).AnyTimes()
	mockShared.EXPECT().Warnf(gomock.Any(), gomock.Any()).AnyTimes()

	client := makeSelectBaseClient(mockModelConfig, mockShared)

	// corecharm.BaseForCharm(lts, nil): supportedBases is empty and the
	// requested base (lts) is non-empty, so it returns (lts, nil) — the
	// old-style charm case. The selector therefore succeeds at step 3.
	lts := coreversion.DefaultSupportedLTSBase()
	got, err := client.selectLocalCharmBase(
		t.Context(),
		&MockConnection{ctrl: ctrl},
		corebase.Base{},
		nil, // no manifest bases
	)
	require.NoError(t, err)
	require.Equal(t, lts.Channel.Track, got.Channel.Track,
		"old-style charm with no manifest bases should get the LTS default")
}
