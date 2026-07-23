// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package juju

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	jujuerrors "github.com/juju/errors"
	"github.com/juju/juju/api"
	apiapplication "github.com/juju/juju/api/client/application"
	apicommoncharm "github.com/juju/juju/api/common/charm"
	"github.com/juju/juju/cmd/juju/application/utils"
	corebase "github.com/juju/juju/core/base"
	corecharm "github.com/juju/juju/core/charm"
	"github.com/juju/juju/core/constraints"
	coreversion "github.com/juju/juju/core/version"
	"github.com/juju/juju/domain/deployment/charm"
	"github.com/juju/juju/environs/config"
	goyaml "gopkg.in/yaml.v2"
)

const LocalCharmOriginHashFirstAgentVersion = "3.6.26"

// LocalCharmInfo describes a local charm archive on disk, used by the provider
// to detect changes to a locally-deployed charm without contacting the
// controller.
type LocalCharmInfo struct {
	// Name is the charm name taken from the archive's metadata.
	Name string
	// Hash is the SHA-256 of the charm archive file contents. It changes
	// whenever the charm file is rebuilt with different content.
	Hash string
	// SupportedBases is the list of bases declared in the archive's
	// manifest.yaml, formatted as "os@channel" (e.g. "ubuntu@22.04").
	// An empty slice means the manifest declares no bases.
	SupportedBases []corebase.Base
}

// ReadLocalCharmInfo reads the local charm archive at the given path and
// returns its metadata name, content hash, and supported bases. It is used at
// plan time to decide whether a locally-deployed charm needs to be refreshed
// or replaced, and to validate the configured base against the archive.
func ReadLocalCharmInfo(path string) (LocalCharmInfo, error) {
	charmArchive, err := charm.ReadCharmArchive(path)
	if err != nil {
		return LocalCharmInfo{}, jujuerrors.Annotatef(err, "cannot read local charm at %q", path)
	}

	hash, err := hashFile(path)
	if err != nil {
		return LocalCharmInfo{}, err
	}

	var supportedBases []corebase.Base
	if manifest := charmArchive.Manifest(); manifest != nil && len(manifest.Bases) > 0 {
		supportedBases, err = corebase.ParseManifestBases(manifest.Bases)
		if err != nil {
			return LocalCharmInfo{}, jujuerrors.Annotatef(err, "cannot parse manifest bases for local charm at %q", path)
		}
	}

	return LocalCharmInfo{
		Name:           charmArchive.Meta().Name,
		Hash:           hash,
		SupportedBases: supportedBases,
	}, nil
}

// CheckLocalCharmBase returns an error if the given base string is not
// compatible with any of the bases declared in the charm archive's
// manifest.yaml. It returns nil when the archive declares no bases (manifest
// absent or empty) or when the configured base is compatible with at least one
// declared base. Compatibility is checked by OS and channel track only,
// ignoring risk/branch, so "ubuntu@22.04" matches "ubuntu@22.04/stable".
func CheckLocalCharmBase(info LocalCharmInfo, base string) error {
	if len(info.SupportedBases) == 0 {
		return nil
	}
	configured, err := corebase.ParseBaseFromString(base)
	if err != nil {
		return jujuerrors.Annotatef(err, "invalid base %q", base)
	}
	for _, supported := range info.SupportedBases {
		if supported.IsCompatible(configured) {
			return nil
		}
	}
	supported := make([]string, len(info.SupportedBases))
	for i, b := range info.SupportedBases {
		supported[i] = b.String()
	}
	return fmt.Errorf(
		"base %q is not supported by the local charm; the archive declares: %s",
		base, strings.Join(supported, ", "),
	)
}

// hashFile returns the hex-encoded SHA-256 of the file at the given path.
func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", jujuerrors.Annotatef(err, "cannot open local charm at %q", path)
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", jujuerrors.Annotatef(err, "cannot hash local charm at %q", path)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// deployFromPath uploads a local charm archive to the controller and then
// deploys it. This mirrors the behaviour of `juju deploy ./path/to/charm`.
// The charm must be a packed .charm archive; deploying from an unpacked
// directory is not supported by the controller.
func (c applicationsClient) deployFromPath(
	ctx context.Context,
	conn api.Connection,
	applicationAPIClient ApplicationAPIClient,
	resourceAPIClient ResourceAPIClient,
	transformedInput transformedCreateApplicationInput,
) error {
	// The charm archive was read during validateAndTransform. Only packed
	// .charm archives are supported; directories are rejected by
	// AddLocalCharm.
	charmArchive := transformedInput.charmArchive

	// Upload the charm archive and build the deploy origin.
	resultURL, origin, err := c.uploadLocalCharm(
		ctx, conn, charmArchive, transformedInput.charmBase, transformedInput.constraints)
	if err != nil {
		return err
	}

	// Register any resources declared in the charm metadata as pending
	// resources and collect their IDs for the Deploy call. This is shared
	// with the store-charm path; passing every metadata resource lets
	// resources absent from the plan default to the store origin, matching
	// `juju deploy ./path/to/charm`.
	charmID := apiapplication.CharmID{
		URL:    resultURL.String(),
		Origin: origin,
	}
	resourceIDs, err := addPendingResources(
		ctx, transformedInput.applicationName, charmArchive.Meta().Resources,
		transformedInput.resources, charmID, resourceAPIClient)
	if err != nil {
		return fmt.Errorf("%w: %w", newApplicationPartiallyCreatedError(transformedInput.applicationName), err)
	}

	settingsForYaml := map[interface{}]interface{}{transformedInput.applicationName: transformedInput.config}
	configYaml, err := goyaml.Marshal(settingsForYaml)
	if err != nil {
		return jujuerrors.Trace(err)
	}

	c.Tracef("Deploying local charm", map[string]interface{}{"url": resultURL.String()})
	err = applicationAPIClient.Deploy(ctx, apiapplication.DeployArgs{
		CharmID:          charmID,
		CharmOrigin:      origin,
		ApplicationName:  transformedInput.applicationName,
		NumUnits:         transformedInput.units,
		ConfigYAML:       string(configYaml),
		Cons:             transformedInput.constraints,
		Placement:        transformedInput.placement,
		Storage:          transformedInput.storage,
		EndpointBindings: transformedInput.endpointBindings,
		Resources:        resourceIDs,
	})
	if err != nil {
		return jujuerrors.Annotatef(err, "cannot deploy local charm %q", resultURL.Name)
	}

	// Trust is not a Deploy argument; it is applied via the application
	// config after deployment, mirroring the DeployFromRepository path.
	if transformedInput.trust {
		err = c.setTrust(ctx, applicationAPIClient, transformedInput.applicationName, true)
		if err != nil {
			return fmt.Errorf("%w: %w", newApplicationPartiallyCreatedError(transformedInput.applicationName), err)
		}
	}

	// Drift detection needs a controller-reported origin hash.
	// If it's missing, warn and continue without drift detection.
	appName := transformedInput.applicationName
	if _, origin, err := applicationAPIClient.GetCharmURLOrigin(ctx, appName); err != nil {
		c.Warnf("could not check local charm drift detection support",
			map[string]interface{}{"app": appName, "err": err.Error()})
	} else if origin.Hash == "" {
		c.Warnf("out-of-band drift detection is disabled, upgrade to Juju "+
			LocalCharmOriginHashFirstAgentVersion+"+ or Juju 4+ to enable it",
			map[string]interface{}{"app": appName})
	}

	return nil
}

// uploadLocalCharm reads the base from the model when necessary, uploads the
// given charm archive to the controller, and returns the resulting local
// charm URL (with the server-assigned revision) and a matching local origin.
func (c applicationsClient) uploadLocalCharm(
	ctx context.Context,
	conn api.Connection,
	charmArchive *charm.CharmArchive,
	charmBase corebase.Base,
	cons constraints.Value,
) (*charm.URL, apicommoncharm.Origin, error) {
	supportedBases, err := corecharm.ComputedBases(charmArchive)
	if err != nil {
		return nil, apicommoncharm.Origin{}, jujuerrors.Annotate(err, "cannot compute supported bases for local charm")
	}

	// Fetch the model config once and derive both the base fallback and the
	// agent version from it, avoiding a second ModelGet round-trip.
	modelConfig, err := c.modelConfig(ctx, conn)
	if err != nil {
		return nil, apicommoncharm.Origin{}, err
	}

	base, err := selectLocalCharmBase(modelConfig, charmBase, supportedBases)
	if err != nil {
		return nil, apicommoncharm.Origin{}, err
	}

	// Build the local charm URL from the charm metadata and revision.
	curl := &charm.URL{
		Schema:   charm.Local.String(),
		Name:     charmArchive.Meta().Name,
		Revision: charmArchive.Revision(),
	}

	// The agent version is required so the controller can validate that
	// the charm is compatible with the deployed agents.
	agentVersion, ok := modelConfig.AgentVersion()
	if !ok {
		return nil, apicommoncharm.Origin{}, jujuerrors.New("cannot determine model agent version")
	}

	localCharmClient, err := c.getLocalCharmClient(conn)
	if err != nil {
		return nil, apicommoncharm.Origin{}, jujuerrors.Trace(err)
	}

	c.Tracef("Uploading local charm", map[string]interface{}{"name": curl.Name})
	resultURL, err := localCharmClient.AddLocalCharm(curl, charmArchive, false, agentVersion)
	if err != nil {
		return nil, apicommoncharm.Origin{}, jujuerrors.Annotatef(err, "cannot upload local charm %q", curl.Name)
	}

	// Build the origin. Local charms have no channel; the architecture is
	// taken from the constraints (defaulting to the controller's
	// architecture) and the base is set above.
	platform := utils.MakePlatform(cons, base, constraints.Value{})
	origin, err := utils.MakeOrigin(charm.Local, resultURL.Revision, charm.Channel{}, platform)
	if err != nil {
		return nil, apicommoncharm.Origin{}, jujuerrors.Trace(err)
	}
	origin.Base = base

	return resultURL, origin, nil
}

// computeLocalCharmID uploads a new local charm archive and returns the
// CharmID needed to refresh an existing application to it via SetCharm. It is
// the local-charm analogue of computeCharmID.
func (c applicationsClient) computeLocalCharmID(
	ctx context.Context,
	conn api.Connection,
	input *UpdateApplicationInput,
) (apiapplication.CharmID, error) {
	charmArchive, err := charm.ReadCharmArchive(input.CharmLocalPath)
	if err != nil {
		return apiapplication.CharmID{}, jujuerrors.Annotatef(err, "cannot read local charm at %q", input.CharmLocalPath)
	}

	var base corebase.Base
	if input.Base != "" {
		base, err = corebase.ParseBaseFromString(input.Base)
		if err != nil {
			return apiapplication.CharmID{}, err
		}
	}

	resultURL, origin, err := c.uploadLocalCharm(ctx, conn, charmArchive, base, constraints.Value{})
	if err != nil {
		return apiapplication.CharmID{}, err
	}

	return apiapplication.CharmID{
		URL:    resultURL.String(),
		Origin: origin,
	}, nil
}

// selectLocalCharmBase selects the base to use when deploying a local charm,
// mirroring the CLI's corecharm.BaseSelector fallback chain:
//  1. User-supplied base (validated against supported bases).
//  2. Model default-base (if explicitly configured and compatible).
//  3. Juju's default supported LTS base (if compatible).
//  4. First base declared in the charm's manifest.
//
// If the charm declares no bases (old-style charm) and no base was supplied,
// an error is returned.
func selectLocalCharmBase(
	modelConfig *config.Config,
	requested corebase.Base,
	supportedBases []corebase.Base,
) (corebase.Base, error) {
	// Step 1: user-supplied base.
	if !requested.Empty() {
		return corecharm.BaseForCharm(requested, supportedBases)
	}

	// Step 2: model default-base (only when explicitly configured).
	modelDefault, err := optionalModelDefaultBase(modelConfig)
	if err != nil {
		return corebase.Base{}, err
	}
	if !modelDefault.Empty() {
		base, err := corecharm.BaseForCharm(modelDefault, supportedBases)
		if err == nil {
			return base, nil
		}
		// Model default is set but incompatible; fall through.
	}

	// Step 3: Juju's default supported LTS base.
	lts := coreversion.DefaultSupportedLTSBase()
	base, err := corecharm.BaseForCharm(lts, supportedBases)
	if err == nil {
		return base, nil
	}

	// Step 4: first base in the charm's manifest (or error if none declared).
	return corecharm.BaseForCharm(corebase.Base{}, supportedBases)
}

// optionalModelDefaultBase returns the model's configured default base, or an
// empty Base when none is set.
func optionalModelDefaultBase(modelConfig *config.Config) (corebase.Base, error) {
	defaultBase, ok := modelConfig.DefaultBase()
	if !ok || defaultBase == "" {
		return corebase.Base{}, nil
	}
	return corebase.ParseBaseFromString(defaultBase)
}

// modelConfig fetches and parses the model configuration in a single API
// round-trip, so callers can derive several settings (e.g. default base and
// agent version) without repeated ModelGet calls.
func (c applicationsClient) modelConfig(ctx context.Context, conn api.Connection) (*config.Config, error) {
	modelConfigAPIClient := c.getModelConfigAPIClient(conn)
	attrs, err := modelConfigAPIClient.ModelGet(ctx)
	if err != nil {
		return nil, jujuerrors.Annotate(err, "cannot get model config")
	}
	modelConfig, err := config.New(config.UseDefaults, attrs)
	if err != nil {
		return nil, jujuerrors.Trace(err)
	}
	return modelConfig, nil
}

// setTrust applies the trust setting to an application by setting the reserved
// "trust" application config key.
func (c applicationsClient) setTrust(ctx context.Context, applicationAPIClient ApplicationAPIClient, appName string, trust bool) error {
	return applicationAPIClient.SetConfig(ctx, appName, "", map[string]string{
		"trust": strconv.FormatBool(trust),
	})
}
