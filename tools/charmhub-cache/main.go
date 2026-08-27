// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Command charmhub-cache is a caching reverse proxy for the CharmHub refresh
// API and charm blob downloads. It is designed for CI: it makes charm metadata
// and blob fetches deterministic and resilient to CharmHub outages, and its
// on-disk store is trivially cacheable via actions/cache.
//
// Two endpoints are served:
//
//   - POST /v2/charms/refresh: responses are cached by the *resolved* charm
//     identity (name + revision) read back from the response, not by the
//     request. This lets a charm the provider deploys by pinned revision and
//     the same charm the controller resolves by channel converge on one cache
//     entry. A channel->revision index lets channel-only requests resolve
//     offline, and an id->name index lets refresh-by-id requests (e.g. the
//     charmrevisioner worker) resolve offline. On a hit the stored JSON is
//     replayed verbatim; on a miss the request is forwarded upstream, embedded
//     download URLs are rewritten to route through this proxy, and the
//     response is stored. Once warm, the proxy serves entirely offline.
//   - GET /blob/<hash>: serves a cached charm blob from disk. On a miss the
//     blob is fetched from the real upstream CDN URL (following redirects),
//     streamed to disk, and served.
//
// The store layout is:
//
//	<store>/refresh/<key>.json          # response keyed by resolved name@revision
//	<store>/channels/<key>.rev          # channel+base -> resolved revision index
//	<store>/ids/<key>.name              # charmhub id+revision -> name index
//	<store>/blobs/<hash>                # charm blob bytes
//	<store>/blobs/<hash>.url            # the real upstream URL for cold backfill
//
// Because charm revisions are immutable, cache entries never expire.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	// refreshPath is the CharmHub refresh endpoint. Both the provider's
	// direct client and the Juju controller post here.
	refreshPath = "/v2/charms/refresh"

	// blobPathPrefix is the path prefix under which rewritten download URLs
	// are served. The segment after the prefix is the blob's sha256, which
	// doubles as the on-disk filename.
	blobPathPrefix = "/blob/"

	// downloadURLField is the JSON path (within each refresh result) that
	// carries the upstream CDN URL for the charm blob.
	downloadURLField = "url"
)

func main() {
	var (
		addr     = flag.String("addr", "127.0.0.1:8080", "address to listen on")
		storeDir = flag.String("store", "charmhub-cache-store", "directory for the on-disk cache")
		upstream = flag.String("upstream", "https://api.charmhub.io", "upstream CharmHub API base URL")
		baseURL  = flag.String("base-url", "http://127.0.0.1:8080", "externally reachable base URL of this proxy, used to rewrite charm download URLs")
	)
	flag.Parse()

	store, err := newStore(*storeDir)
	if err != nil {
		log.Fatalf("charmhub-cache: open store: %v", err)
	}

	proxy := &server{
		store:    store,
		upstream: strings.TrimRight(*upstream, "/"),
		baseURL:  strings.TrimRight(*baseURL, "/"),
	}

	mux := http.NewServeMux()
	mux.Handle(refreshPath, http.HandlerFunc(proxy.handleRefresh))
	mux.Handle(blobPathPrefix, http.HandlerFunc(proxy.handleBlob))
	// Readiness endpoint: CI polls this (e.g. curl --retry) before pointing
	// the controller and provider at the proxy.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("charmhub-cache: listening on %s (upstream=%s store=%s)", *addr, *upstream, *storeDir)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("charmhub-cache: %v", err)
	}
}

// server implements the caching proxy.
type server struct {
	store    *store
	upstream string
	baseURL  string // externally reachable URL of this proxy, for rewriting download URLs
}

// handleRefresh serves POST /v2/charms/refresh.
//
// The cache is keyed by the *resolved* charm identity (name + revision), not
// the request. This is what makes the proxy serve offline across clients:
// the provider requests a charm by pinned revision (revision=77), while the
// Juju controller resolves the same charm by channel (latest/stable). Both
// converge on the same (name, revision) once Charmhub answers, so we store
// the response under that canonical key and maintain a channel->revision
// index so channel-only requests can be answered offline too.
//
// On a hit the stored response is replayed verbatim. On a miss the request is
// forwarded to upstream; the response is parsed to learn the resolved
// (name, revision), its download URLs are rewritten to route through this
// proxy, and it is stored under both the canonical key and the channel index.
func (s *server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	actions, err := parseRefreshActions(body)
	if err != nil {
		http.Error(w, fmt.Sprintf("parse refresh request: %v", err), http.StatusBadRequest)
		return
	}

	// Try to answer from cache. A request is answerable offline only if every
	// action resolves to a cached entry (directly by revision, or via the
	// channel index). We only support single-action requests for offline
	// replay; multi-action requests fall through to upstream.
	if len(actions) == 1 {
		if cached, ok := s.store.LookupRefresh(actions[0]); ok {
			// Rewrite the instance-key in the cached response to match the
			// current request. The instance-key is a per-request UUID; the
			// cached response carries the original request's key, which won't
			// match and causes the client's Ensure() to reject it.
			replayed := rewriteInstanceKey(cached, actions[0].InstanceKey)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Charmhub-Cache", "HIT")
			_, _ = w.Write(replayed)
			return
		}
	}

	// Cache miss: forward to upstream and capture the response.
	upstreamURL := s.upstream + refreshPath
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, upstreamURL, bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("build upstream request: %v", err), http.StatusBadGateway)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream unreachable and no cache entry: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read upstream: %v", err), http.StatusBadGateway)
		return
	}

	if resp.StatusCode != http.StatusOK {
		// Pass non-200 responses through unmodified; they are not cached.
		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(respBody)
		return
	}

	// Rewrite download URLs so blob fetches route back through this proxy,
	// then store under the resolved identity and return the rewritten bytes.
	rewritten, err := rewriteDownloadURLs(respBody, s.store, s.baseURL)
	if err != nil {
		// Rewriting failed: fall back to returning the original so we never
		// break a deploy because of a schema surprise. The blob path will
		// then hit the live CDN directly.
		log.Printf("charmhub-cache: rewrite failed (%v); returning raw response", err)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Charmhub-Cache", "MISS")
		_, _ = w.Write(respBody)
		return
	}

	// Store under the resolved (name, revision) and record the channel index
	// so future channel-only requests resolve offline. Best-effort: a store
	// failure must not fail the deploy.
	if err := s.store.PutRefresh(actions, rewritten); err != nil {
		log.Printf("charmhub-cache: store refresh failed: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Charmhub-Cache", "MISS")
	_, _ = w.Write(rewritten)
}

// handleBlob serves GET /blob/<hash>.
//
// The <hash> is the charm blob's sha256 (as reported by the refresh
// response's download.hash-sha-256). On a miss the blob is fetched from the
// real upstream URL recorded when the refresh response was rewritten.
func (s *server) handleBlob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	hash := strings.TrimPrefix(r.URL.Path, blobPathPrefix)
	if hash == "" || !isHex(hash) {
		http.Error(w, "bad blob id", http.StatusBadRequest)
		return
	}

	// Fast path: serve from disk.
	if f, ok := s.store.OpenBlob(hash); ok {
		defer f.Close()
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("X-Charmhub-Cache", "HIT")
		_, _ = io.Copy(w, f)
		return
	}

	// Miss: look up the real upstream URL we recorded for this hash.
	realURL, ok := s.store.BlobURL(hash)
	if !ok {
		http.Error(w, "blob not found and no upstream URL recorded", http.StatusNotFound)
		return
	}

	// Fetch from the upstream CDN, following redirects, streaming to disk
	// and to the client simultaneously. fetchAndStoreBlob reports whether it
	// began writing the body; if it failed before writing anything we can
	// still send an error status, otherwise the response is already committed.
	wrote, err := s.fetchAndStoreBlob(r.Context(), hash, realURL, w)
	if err != nil {
		log.Printf("charmhub-cache: blob fetch %s failed: %v", hash, err)
		if !wrote {
			http.Error(w, fmt.Sprintf("blob fetch failed: %v", err), http.StatusBadGateway)
		}
	}
}

// fetchAndStoreBlob downloads the blob from the upstream CDN URL, teeing the
// bytes to both the on-disk store and the HTTP response. It returns whether
// any response bytes were written, so the caller can decide if an error
// status can still be sent.
func (s *server) fetchAndStoreBlob(ctx context.Context, hash, realURL string, w http.ResponseWriter) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, realURL, nil)
	if err != nil {
		return false, fmt.Errorf("build request: %w", err)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("upstream status %d", resp.StatusCode)
	}

	// Tee into the store while streaming to the client. The store writer is
	// buffered to a temp file and renamed on close, so a partial download
	// never leaves a corrupt entry.
	sink, commit, err := s.store.BeginBlob(hash)
	if err != nil {
		return false, fmt.Errorf("begin store: %w", err)
	}
	defer sink.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Charmhub-Cache", "MISS")
	if n, ok := contentLength(resp); ok {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", n))
	}

	mw := io.MultiWriter(w, sink)
	if _, err := io.Copy(mw, resp.Body); err != nil {
		return true, fmt.Errorf("copy: %w", err)
	}
	if err := commit(); err != nil {
		return true, fmt.Errorf("commit: %w", err)
	}
	return true, nil
}

func contentLength(resp *http.Response) (int64, bool) {
	if resp.ContentLength > 0 {
		return resp.ContentLength, true
	}
	return 0, false
}

// action is a single parsed refresh request action. Only the fields needed
// for cache keying are captured.
//
// Two wire shapes exist:
//   - install/download-by-name (provider, controller fresh deploy): the action
//     carries Name and optional Revision/Channel/Base.
//   - refresh-by-id (controller re-resolve, charmrevisioner): the action
//     carries only ID; Revision/Channel/Base live in the matching context
//     entry (keyed by instance-key).
type action struct {
	ID          string // charmhub charm ID (refresh-by-id); empty for by-name
	Name        string // charm name (by-name); empty for by-id
	Channel     string
	Revision    *int
	Base        string // "arch/os/channel", empty if absent
	InstanceKey string // per-request unique key; must match in the response
}

// parseRefreshActions extracts the actions from a refresh request body,
// merging in revision/channel/base from the context array for refresh-by-id
// requests (matched by instance-key). It returns an error only if the body is
// not valid JSON; a request with no actions yields an empty slice.
func parseRefreshActions(body []byte) ([]action, error) {
	var req struct {
		Context []struct {
			InstanceKey     string `json:"instance-key"`
			Revision        *int   `json:"revision"`
			TrackingChannel string `json:"tracking-channel"`
			Base            *struct {
				Architecture string `json:"architecture"`
				Name         string `json:"name"`
				Channel      string `json:"channel"`
			} `json:"base"`
		} `json:"context"`
		Actions []struct {
			InstanceKey string `json:"instance-key"`
			ID          string `json:"id"`
			Name        string `json:"name"`
			Channel     string `json:"channel"`
			Revision    *int   `json:"revision"`
			Base        *struct {
				Architecture string `json:"architecture"`
				Name         string `json:"name"`
				Channel      string `json:"channel"`
			} `json:"base"`
		} `json:"actions"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	// Index context entries by instance-key so refresh-by-id actions can pick
	// up their revision/channel/base.
	type ctxInfo struct {
		revision *int
		channel  string
		base     string
	}
	ctxByKey := make(map[string]ctxInfo, len(req.Context))
	for _, c := range req.Context {
		base := ""
		if c.Base != nil {
			base = fmt.Sprintf("%s/%s/%s", c.Base.Architecture, c.Base.Name, c.Base.Channel)
		}
		ctxByKey[c.InstanceKey] = ctxInfo{revision: c.Revision, channel: c.TrackingChannel, base: base}
	}

	actions := make([]action, 0, len(req.Actions))
	for _, a := range req.Actions {
		act := action{ID: a.ID, Name: a.Name, Channel: a.Channel, Revision: a.Revision, InstanceKey: a.InstanceKey}
		if a.Base != nil {
			act.Base = fmt.Sprintf("%s/%s/%s", a.Base.Architecture, a.Base.Name, a.Base.Channel)
		}
		// For refresh-by-id, revision/channel/base come from the context.
		if ci, ok := ctxByKey[a.InstanceKey]; ok {
			if act.Revision == nil {
				act.Revision = ci.revision
			}
			if act.Channel == "" {
				act.Channel = ci.channel
			}
			if act.Base == "" {
				act.Base = ci.base
			}
		}
		actions = append(actions, act)
	}
	return actions, nil
}

// rewriteDownloadURLs rewrites the download.url field in each refresh result
// to point at this proxy's /blob/<hash> endpoint, recording the real upstream
// rewriteInstanceKey replaces the instance-key in each refresh result with the
// given key. The instance-key is a per-request UUID; a cached response carries
// the original request's key, which won't match the current request and causes
// the client's Ensure() to reject it ("install action key not valid"). Uses
// map[string]any to preserve all other fields verbatim. If parsing fails the
// original body is returned unchanged (best-effort).
func rewriteInstanceKey(body []byte, instanceKey string) []byte {
	if instanceKey == "" {
		return body
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return body
	}
	results, ok := doc["results"].([]any)
	if !ok {
		return body
	}
	for _, res := range results {
		result, ok := res.(map[string]any)
		if !ok {
			continue
		}
		result["instance-key"] = instanceKey
	}
	rewritten, err := json.Marshal(doc)
	if err != nil {
		return body
	}
	return rewritten
}

// rewriteDownloadURLs rewrites the download.url field in each refresh result
// to point at this proxy's /blob/<hash> endpoint, recording the real upstream
// URL for later backfill. It operates on generic JSON (map[string]any) so
// that fields not modelled by the transport package are preserved verbatim.
// proxyBase is the externally reachable URL of this proxy (e.g.
// http://127.0.0.1:8080); Juju's charm downloader requires absolute URLs.
func rewriteDownloadURLs(body []byte, st *store, proxyBase string) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal refresh response: %w", err)
	}

	results, ok := doc["results"].([]any)
	if !ok {
		// No results array (e.g. an error-list response): nothing to rewrite.
		return body, nil
	}

	for _, res := range results {
		result, ok := res.(map[string]any)
		if !ok {
			continue
		}
		entity, ok := result["charm"].(map[string]any)
		if !ok {
			continue
		}
		download, ok := entity["download"].(map[string]any)
		if !ok {
			continue
		}
		realURL, _ := download[downloadURLField].(string)
		if realURL == "" {
			continue
		}
		hash, _ := download["hash-sha-256"].(string)
		if hash == "" {
			// Fall back to a hash of the URL so we still have a stable key.
			sum := sha256.Sum256([]byte(realURL))
			hash = hex.EncodeToString(sum[:])
		}
		// Record the real URL so a blob miss can backfill from upstream.
		if err := st.PutBlobURL(hash, realURL); err != nil {
			return nil, fmt.Errorf("record blob url: %w", err)
		}
		// Rewrite the URL to an absolute URL routing through this proxy.
		// Juju's charm downloader requires an absolute URL.
		download[downloadURLField] = proxyBase + blobPathPrefix + hash
	}

	return json.Marshal(doc)
}

// httpClient is used for all upstream fetches. It follows redirects (the
// CharmHub blob CDN issues 302s) and uses a generous timeout suitable for
// large charm blobs.
var httpClient = &http.Client{
	Timeout:       5 * time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
}

// isHex reports whether s is a non-empty lowercase hex string.
func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		isDigit := r >= '0' && r <= '9'
		isLowerHex := r >= 'a' && r <= 'f'
		if !isDigit && !isLowerHex {
			return false
		}
	}
	return true
}

// --- store -----------------------------------------------------------------

type store struct {
	dir       string
	refreshMu sync.Mutex
	blobMu    sync.Mutex
}

func newStore(dir string) (*store, error) {
	for _, sub := range []string{"refresh", "blobs", "channels", "ids"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &store{dir: dir}, nil
}

// canonicalKey returns the on-disk key for a resolved charm identity. The
// response is stored under this key so that any request resolving to the same
// (name, revision) — whether by pinned revision or by channel — hits it.
func canonicalKey(name string, revision int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s@%d", name, revision)))
	return hex.EncodeToString(sum[:])
}

// channelKey returns the on-disk key for the channel->revision index entry.
func channelKey(name, channel, base string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s", name, channel, base)))
	return hex.EncodeToString(sum[:])
}

// LookupRefresh returns the cached response for a single action. It resolves
//   - pinned-revision requests directly via the canonical (name, revision) key,
//   - channel-only requests via the channel->revision index, then canonical,
//   - refresh-by-id requests (charmrevisioner / re-resolve) via the id index,
//     then the revision-keyed canonical entry for the SAME id+revision.
//
// It returns false if the action cannot be answered from cache.
func (s *store) LookupRefresh(a action) ([]byte, bool) {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	// refresh-by-id: the action carries the charmhub ID, not the name. Resolve
	// the id to its canonical (name, revision) via the ids index. The id index
	// stores "name@revision" keyed by id+revision so the same id at a new
	// revision is a distinct entry.
	if a.Name == "" && a.ID != "" {
		if a.Revision == nil {
			return nil, false // can't key an id lookup without a revision
		}
		nameBytes, err := os.ReadFile(filepath.Join(s.dir, "ids", idKey(a.ID, *a.Revision)+".name"))
		if err != nil {
			return nil, false
		}
		name := strings.TrimSpace(string(nameBytes))
		if name == "" {
			return nil, false
		}
		b, err := os.ReadFile(filepath.Join(s.dir, "refresh", canonicalKey(name, *a.Revision)+".json"))
		if err != nil {
			return nil, false
		}
		return b, true
	}

	var key string
	if a.Revision != nil {
		// Pinned revision: direct canonical lookup.
		key = canonicalKey(a.Name, *a.Revision)
	} else if a.Channel != "" {
		// Channel-only: resolve via the channel index to a revision, then
		// look up the canonical entry.
		revBytes, err := os.ReadFile(filepath.Join(s.dir, "channels", channelKey(a.Name, a.Channel, a.Base)+".rev"))
		if err != nil {
			return nil, false
		}
		var rev int
		if _, err := fmt.Sscanf(strings.TrimSpace(string(revBytes)), "%d", &rev); err != nil {
			return nil, false
		}
		key = canonicalKey(a.Name, rev)
	} else {
		return nil, false
	}

	b, err := os.ReadFile(filepath.Join(s.dir, "refresh", key+".json"))
	if err != nil {
		return nil, false
	}
	return b, true
}

// idKey returns the on-disk key for the id->name index, scoped by revision so
// the same charm id at different revisions maps to distinct canonical entries.
func idKey(id string, revision int) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s@%d", id, revision)))
	return hex.EncodeToString(sum[:])
}

// PutRefresh stores a refresh response under the resolved (name, revision)
// identity and records the channel->revision index for each action that
// specified a channel. The resolved identity is read back from the response
// body (each result's charm.name and charm.revision), so the store reflects
// what Charmhub actually resolved to.
func (s *store) PutRefresh(actions []action, body []byte) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	resolved, err := resolvedIdentities(body)
	if err != nil {
		return fmt.Errorf("parse resolved identities: %w", err)
	}
	if len(resolved) == 0 {
		return errors.New("no resolved results in response")
	}

	// Store the response under the canonical key of the first result. The
	// proxy only caches single-action requests, so there is exactly one.
	name, rev := resolved[0].name, resolved[0].revision
	path := filepath.Join(s.dir, "refresh", canonicalKey(name, rev)+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	// Record the id->name index so refresh-by-id requests (charmrevisioner,
	// re-resolve) resolve offline. The id comes from the response entity.
	if id := resolved[0].id; id != "" {
		idPath := filepath.Join(s.dir, "ids", idKey(id, rev)+".name")
		if err := os.WriteFile(idPath, []byte(name), 0o644); err != nil {
			return err
		}
	}

	// Record the channel index for any action that named a channel, so
	// future channel-only requests for the same channel/base resolve offline.
	for _, a := range actions {
		if a.Channel == "" {
			continue
		}
		// For by-name actions use the action's name; for by-id actions use the
		// resolved name (the action has no name).
		chName := a.Name
		if chName == "" {
			chName = name
		}
		revPath := filepath.Join(s.dir, "channels", channelKey(chName, a.Channel, a.Base)+".rev")
		if err := os.WriteFile(revPath, []byte(fmt.Sprintf("%d", rev)), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// resolvedIdentity is the (id, name, revision) a refresh result resolved to.
type resolvedIdentity struct {
	id       string
	name     string
	revision int
}

// resolvedIdentities parses a refresh response and returns the resolved
// (id, name, revision) for each result.
func resolvedIdentities(body []byte) ([]resolvedIdentity, error) {
	var doc struct {
		Results []struct {
			Entity struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Revision int    `json:"revision"`
			} `json:"charm"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	out := make([]resolvedIdentity, 0, len(doc.Results))
	for _, r := range doc.Results {
		if r.Entity.Name == "" {
			continue
		}
		out = append(out, resolvedIdentity{id: r.Entity.ID, name: r.Entity.Name, revision: r.Entity.Revision})
	}
	return out, nil
}

// OpenBlob opens a cached blob for reading. The caller must close the file.
func (s *store) OpenBlob(hash string) (*os.File, bool) {
	path := filepath.Join(s.dir, "blobs", hash)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	return f, true
}

// BlobURL returns the recorded upstream URL for a blob hash.
func (s *store) BlobURL(hash string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(s.dir, "blobs", hash+".url"))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

// PutBlobURL records the upstream URL for a blob hash.
func (s *store) PutBlobURL(hash, url string) error {
	path := filepath.Join(s.dir, "blobs", hash+".url")
	return os.WriteFile(path, []byte(url), 0o644)
}

// BeginBlob returns a writer that buffers the blob to a unique temp file.
// Call commit to atomically promote it to its final name. A unique temp name
// (via os.CreateTemp) prevents two concurrent fetches of the same blob from
// truncating each other's writes.
func (s *store) BeginBlob(hash string) (sink io.WriteCloser, commit func() error, err error) {
	s.blobMu.Lock()
	defer s.blobMu.Unlock()
	final := filepath.Join(s.dir, "blobs", hash)
	f, err := os.CreateTemp(filepath.Join(s.dir, "blobs"), hash+".tmp.*")
	if err != nil {
		return nil, nil, err
	}
	tmp := f.Name()
	commit = func() error {
		if err := f.Sync(); err != nil {
			return err
		}
		return os.Rename(tmp, final)
	}
	return f, commit, nil
}
