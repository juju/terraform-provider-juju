// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Package charmhub provides a minimal HTTP client for the CharmHub refresh endpoint.
package charmhub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/juju/terraform-provider-juju/internal/charmhub/transport"

	"github.com/juju/charm/v12"
	"github.com/juju/errors"
)

const (
	// ProductionURL is the base URL of the production CharmHub API.
	ProductionURL = "https://api.charmhub.io"

	refreshPath    = "/v2/charms/refresh"
	defaultTimeout = 30 * time.Second
	defaultArch    = "amd64"
)

var refreshFields = []string{"bases", "metadata-yaml", "name", "resources", "revision"}

// CharmRefreshInput contains the parameters for a charm refresh request.
type CharmRefreshInput struct {
	Name         string
	Base         string // "os@channel", e.g. "ubuntu@22.04"
	Architecture string // defaults to "amd64"
	Channel      string
	Revision     *int // nil = latest in channel; non-nil requires Channel to be set
}

// CharmRefreshResult contains the results of a charm refresh request.
type CharmRefreshResult struct {
	Name      string
	Channel   string
	Base      string
	Revision  int
	Resources []transport.ResourceRevision
	Provides  map[string]charm.Relation
	Requires  map[string]charm.Relation
}

// Client is the CharmHub API client.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// New returns a Client targeting baseURL. Pass a custom *http.Client as hc to
// control timeouts, TLS config, etc.; pass nil to use a sensible default.
func New(baseURL string, hc *http.Client) *Client {
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	return &Client{baseURL: baseURL, httpClient: hc}
}

// Refresh posts a request to the CharmHub refresh endpoint and returns the results.
func (c *Client) Refresh(ctx context.Context, input CharmRefreshInput) (*CharmRefreshResult, error) {
	arch := input.Architecture
	if arch == "" {
		arch = defaultArch
	}

	action := transport.RefreshRequestAction{
		Action:      "install",
		InstanceKey: "key-0",
		Name:        &input.Name,
		Revision:    input.Revision,
	}
	// When revision is not set, we need to specify both channel and base.
	// When revision is set, channel and base must be nil.
	if input.Revision == nil {
		parts := strings.SplitN(input.Base, "@", 2)
		osName, osCh := parts[0], "NA"
		if len(parts) == 2 {
			osCh = parts[1]
		}
		base := transport.Base{Architecture: arch, Name: osName, Channel: osCh}
		action.Channel = &input.Channel
		action.Base = &base
	}

	req := transport.RefreshRequest{
		Context: []transport.RefreshRequestContext{},
		Actions: []transport.RefreshRequestAction{action},
		Fields:  refreshFields,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Errorf("charmhub: %s", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+refreshPath, bytes.NewReader(body))
	if err != nil {
		return nil, errors.Errorf("charmhub: %s", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, errors.Errorf("charmhub: %s", err)
	}
	defer httpResp.Body.Close()

	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, errors.Errorf("charmhub: %s", err)
	}
	if httpResp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("charmhub: status %d: %s", httpResp.StatusCode, string(data))
	}

	var resp transport.RefreshResponses
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, errors.Errorf("charmhub: %s", err)
	}
	if len(resp.ErrorList) > 0 {
		return nil, errors.Errorf("charmhub: %s", resp.ErrorList)
	}
	if len(resp.Results) == 0 {
		return nil, errors.Errorf("charmhub: no results")
	}
	r := resp.Results[0]
	if r.Error != nil {
		return nil, fmt.Errorf("charmhub: %s: %s", r.Error.Code, r.Error.Message)
	}

	result := &CharmRefreshResult{
		Name:      r.Name,
		Channel:   r.EffectiveChannel,
		Revision:  r.Entity.Revision,
		Resources: r.Entity.Resources,
	}
	if len(r.Entity.Bases) > 0 {
		b := r.Entity.Bases[0]
		result.Base = fmt.Sprintf("%s@%s", b.Name, b.Channel)
	}
	if r.Entity.MetadataYAML != "" {
		meta, err := charm.ReadMeta(strings.NewReader(r.Entity.MetadataYAML))
		if err != nil {
			return nil, errors.Errorf("charmhub: parse metadata.yaml for %q: %s", r.Name, err)
		}
		result.Provides = meta.Provides
		result.Requires = meta.Requires
	}
	return result, nil
}
