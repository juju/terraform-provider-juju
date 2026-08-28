// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

// Command charmhub-cache is a caching reverse proxy for the CharmHub refresh
// API and charm blob downloads, designed for CI. Responses are cached by the
// resolved charm identity (name + revision), which is immutable, so entries
// never expire. Requests pinned to a revision (or an id+revision) are served
// from cache; channel-only requests always go upstream, since their entire
// purpose is to learn the CURRENT revision for a channel, which can change at
// any time — caching that would serve stale revisions.
//
// Store layout:
//
//	<store>/refresh/<key>.json   # response keyed by resolved name@revision
//	<store>/ids/<key>.name       # charmhub id+revision -> name index
//	<store>/blobs/<hash>         # charm blob bytes
//	<store>/blobs/<hash>.url     # real upstream URL for cold backfill
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
	refreshPath = "/v2/charms/refresh"

	// blobPathPrefix is where rewritten download URLs are served; the
	// segment after it is the blob's sha256, doubling as the on-disk name.
	blobPathPrefix = "/blob/"
)

func main() {
	var (
		addr     = flag.String("addr", "127.0.0.1:8080", "address to listen on")
		storeDir = flag.String("store", "charmhub-cache-store", "directory for the on-disk cache")
		upstream = flag.String("upstream", "https://api.charmhub.io", "upstream CharmHub API base URL")
	)
	flag.Parse()

	store, err := newStore(*storeDir)
	if err != nil {
		log.Fatalf("charmhub-cache: open store: %v", err)
	}

	proxy := &server{
		store:    store,
		upstream: strings.TrimRight(*upstream, "/"),
	}

	mux := http.NewServeMux()
	mux.Handle(refreshPath, http.HandlerFunc(proxy.handleRefresh))
	mux.Handle(blobPathPrefix, http.HandlerFunc(proxy.handleBlob))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// Anything else under the Charmhub API (e.g. resource revision lookups)
	// isn't cached; pass it straight through to upstream so the client sees a
	// real Charmhub response instead of this proxy's 404.
	mux.Handle("/", http.HandlerFunc(proxy.handlePassthrough))

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
}

// proxyBaseURL derives this proxy's externally reachable base URL from the
// incoming request, so no --base-url flag or restart is needed when the
// controller-reachable address only becomes known later (e.g. once the LXD
// bridge exists). Whatever host:port the client used to reach us (via
// charmhub-url/CHARMHUB_URL) is what must be embedded in rewritten URLs.
func proxyBaseURL(r *http.Request) string {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

// handlePassthrough forwards any request not otherwise cached (e.g.
// /v2/charms/resources/<charm>/<resource>/revisions) straight to upstream,
// preserving method, headers, and body. Not cached: these are outside the
// refresh/blob paths this proxy understands.
func (s *server) handlePassthrough(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("read body: %v", err), http.StatusBadRequest)
		return
	}
	_ = r.Body.Close()

	req, err := http.NewRequestWithContext(r.Context(), r.Method, s.upstream+r.URL.RequestURI(), bytes.NewReader(body))
	if err != nil {
		http.Error(w, fmt.Sprintf("build upstream request: %v", err), http.StatusBadGateway)
		return
	}
	for k, v := range r.Header {
		req.Header[k] = v
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("upstream unreachable: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// handleRefresh serves POST /v2/charms/refresh. See the package comment for
// the caching strategy.
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

	// Only single-action requests are replayed from cache; multi-action
	// requests fall through to upstream.
	if len(actions) == 1 {
		if cached, ok := s.store.LookupRefresh(actions[0]); ok {
			// The instance-key is a per-request UUID; the cached response
			// carries the original request's key, which the client's
			// Ensure() would reject. Rewrite it to match.
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

	// Rewrite download URLs so blob fetches route back through this proxy.
	rewritten, err := rewriteDownloadURLs(respBody, s.store, proxyBaseURL(r))
	if err != nil {
		// Never break a deploy over a schema surprise; the blob path will
		// hit the live CDN directly.
		log.Printf("charmhub-cache: rewrite failed (%v); returning raw response", err)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Charmhub-Cache", "MISS")
		_, _ = w.Write(respBody)
		return
	}

	// Best-effort: a store failure must not fail the deploy.
	if err := s.store.PutRefresh(rewritten); err != nil {
		log.Printf("charmhub-cache: store refresh failed: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Charmhub-Cache", "MISS")
	_, _ = w.Write(rewritten)
}

// handleBlob serves GET /blob/<hash>, where <hash> is the blob's sha256 as
// reported by the refresh response's download.hash-sha-256.
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

	// fetchAndStoreBlob reports whether it began writing the body; if it
	// failed before writing anything we can still send an error status.
	wrote, err := s.fetchAndStoreBlob(r.Context(), hash, realURL, w)
	if err != nil {
		log.Printf("charmhub-cache: blob fetch %s failed: %v", hash, err)
		if !wrote {
			http.Error(w, fmt.Sprintf("blob fetch failed: %v", err), http.StatusBadGateway)
		}
	}
}

// fetchAndStoreBlob downloads the blob, teeing bytes to the store and the
// client. It returns whether any response bytes were written.
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

	// Buffered to a temp file and renamed on commit, so a partial download
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

// action is a single parsed refresh request action.
//
// Two wire shapes exist:
//   - by-name (provider, controller fresh deploy): Name plus optional
//     Revision/Channel/Base in the action.
//   - by-id (controller re-resolve, charmrevisioner): only ID in the action;
//     Revision/Channel/Base live in the matching context entry.
type action struct {
	ID          string
	Name        string
	Channel     string
	Revision    *int
	Base        string // "arch/os/channel", empty if absent
	InstanceKey string // per-request unique key; must match in the response
}

// parseRefreshActions extracts the actions from a refresh request body,
// merging revision/channel/base from the context array (matched by
// instance-key) for by-id requests.
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

	// Index context entries by instance-key so by-id actions can pick up
	// their revision/channel/base.
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
		// For by-id requests, revision/channel/base come from the context.
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

// rewriteInstanceKey replaces the instance-key in each result with the given
// key, so a replayed response passes the client's Ensure() check. Uses
// map[string]any to preserve all other fields verbatim; on any parse failure
// the original body is returned unchanged.
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

// rewriteDownloadURLs rewrites each result's download.url to point at this
// proxy's /blob/<hash> endpoint, recording the real upstream URL for later
// backfill. Uses map[string]any so unmodelled fields are preserved verbatim.
// Juju's charm downloader requires absolute URLs, hence proxyBase.
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
		realURL, _ := download["url"].(string)
		if realURL == "" {
			continue
		}
		hash, _ := download["hash-sha-256"].(string)
		if hash == "" {
			sum := sha256.Sum256([]byte(realURL))
			hash = hex.EncodeToString(sum[:])
		}
		if err := st.PutBlobURL(hash, realURL); err != nil {
			return nil, fmt.Errorf("record blob url: %w", err)
		}
		download["url"] = proxyBase + blobPathPrefix + hash
	}

	return json.Marshal(doc)
}

// httpClient follows redirects (the CharmHub blob CDN issues 302s) and uses
// a generous timeout suitable for large charm blobs.
var httpClient = &http.Client{
	Timeout:       5 * time.Minute,
	CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
}

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
	for _, sub := range []string{"refresh", "blobs", "ids"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &store{dir: dir}, nil
}

// canonicalKey returns the on-disk key for a resolved charm identity.
func canonicalKey(name string, revision int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s@%d", name, revision))
	return hex.EncodeToString(sum[:])
}

func idKey(id string, revision int) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s@%d", id, revision))
	return hex.EncodeToString(sum[:])
}

// LookupRefresh returns the cached response for a single action, resolving:
//   - pinned-revision requests via the canonical (name, revision) key,
//   - by-id requests via the id->name index (requires a revision).
//
// Channel-only requests (no revision) are never served from cache: a
// channel's resolved revision can change at any time, so caching it would
// serve stale revisions once the channel advances upstream.
func (s *store) LookupRefresh(a action) ([]byte, bool) {
	if a.Revision == nil {
		return nil, false
	}

	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if a.Name == "" && a.ID != "" {
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

	b, err := os.ReadFile(filepath.Join(s.dir, "refresh", canonicalKey(a.Name, *a.Revision)+".json"))
	if err != nil {
		return nil, false
	}
	return b, true
}

// PutRefresh stores a response under the resolved (name, revision) identity
// read back from the response body, and records the id index so future by-id
// requests for the same immutable id+revision resolve offline.
func (s *store) PutRefresh(body []byte) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	resolved, err := resolvedIdentities(body)
	if err != nil {
		return fmt.Errorf("parse resolved identities: %w", err)
	}
	if len(resolved) == 0 {
		return errors.New("no resolved results in response")
	}

	// Only single-action requests are cached, so there is exactly one result.
	name, rev := resolved[0].name, resolved[0].revision
	path := filepath.Join(s.dir, "refresh", canonicalKey(name, rev)+".json")
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, body, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}

	if id := resolved[0].id; id != "" {
		idPath := filepath.Join(s.dir, "ids", idKey(id, rev)+".name")
		if err := os.WriteFile(idPath, []byte(name), 0o644); err != nil {
			return err
		}
	}
	return nil
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

type resolvedIdentity struct {
	id       string
	name     string
	revision int
}

func (s *store) OpenBlob(hash string) (*os.File, bool) {
	path := filepath.Join(s.dir, "blobs", hash)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	return f, true
}

func (s *store) BlobURL(hash string) (string, bool) {
	b, err := os.ReadFile(filepath.Join(s.dir, "blobs", hash+".url"))
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(b)), true
}

func (s *store) PutBlobURL(hash, url string) error {
	path := filepath.Join(s.dir, "blobs", hash+".url")
	return os.WriteFile(path, []byte(url), 0o644)
}

// BeginBlob returns a writer that buffers the blob to a unique temp file;
// call commit to atomically promote it to its final name. A unique temp name
// prevents concurrent fetches of the same blob from truncating each other.
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
