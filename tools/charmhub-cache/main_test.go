// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// refreshResponse builds a minimal Charmhub refresh response body for the
// given resolved charm, with an id, a download URL and hash.
func refreshResponse(name string, revision int, hash, url string) []byte {
	return []byte(`{"results":[{"charm":{"id":"charm-id-` + name + `","name":"` + name + `","revision":` +
		strconv.Itoa(revision) + `,"download":{"hash-sha-256":"` + hash + `","size":1,"url":"` + url + `"}},"effective-channel":"latest/stable"}]}`)
}

// multiResultRefreshResponse builds a batched refresh response containing
// results for two charms, as the charmrevisioner's RefreshMany can produce.
func multiResultRefreshResponse(a, b string, revA, revB int) []byte {
	one := `{"charm":{"id":"charm-id-` + a + `","name":"` + a + `","revision":` + strconv.Itoa(revA) + `},"effective-channel":"latest/stable"}`
	two := `{"charm":{"id":"charm-id-` + b + `","name":"` + b + `","revision":` + strconv.Itoa(revB) + `},"effective-channel":"latest/stable"}`
	return []byte(`{"results":[` + one + `,` + two + `]}`)
}

// TestPutRefreshRejectsMultiResult is a regression test: caching a batched
// (multi-result) response under one canonical key would later be replayed
// verbatim to an unrelated single-action request expecting exactly one
// result. The Juju client rejects that with "more than 1 result found"
// (core/charm/repository.(*CharmHubRepository).refreshOne requires len==1).
// PutRefresh must refuse to store multi-result bodies.
func TestPutRefreshRejectsMultiResult(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	body := multiResultRefreshResponse("ubuntu", "postgresql", 77, 10)
	if err := st.PutRefresh(body); err == nil {
		t.Fatalf("PutRefresh accepted a multi-result response; must reject it")
	}

	// Confirm nothing was cached: a later single-action lookup for either
	// charm must miss (go to upstream), not return the batched body.
	rev := 77
	if _, ok := st.LookupRefresh(action{Name: "ubuntu", Revision: &rev}); ok {
		t.Fatalf("multi-result response was cached despite PutRefresh rejecting it")
	}
}

// TestCrossClientConvergence is the core offline guarantee for pinned
// revisions: once a (name, revision) has been resolved once, any later
// request pinned to that SAME revision hits the same cache entry, regardless
// of which client asked. Channel-only requests are never cached (see
// TestChannelOnlyNeverCached) since the resolved revision can change.
func TestCrossClientConvergence(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// Charmhub answers a resolve for ubuntu with revision 77.
	resp := refreshResponse("ubuntu", 77, "abc123", "https://cdn.example/ubuntu_77.charm")
	if err := st.PutRefresh(resp); err != nil {
		t.Fatalf("PutRefresh: %v", err)
	}

	// A later pinned-revision request for the SAME charm+revision hits the
	// SAME canonical entry, regardless of which client asked.
	rev := 77
	providerAction := action{Name: "ubuntu", Revision: &rev}
	got, ok := st.LookupRefresh(providerAction)
	if !ok {
		t.Fatalf("pinned-revision request did not resolve from cache")
	}
	if string(got) != string(resp) {
		t.Fatalf("provider got different body than stored")
	}

	// A different revision is a distinct entry (miss).
	rev78 := 78
	if _, ok := st.LookupRefresh(action{Name: "ubuntu", Revision: &rev78}); ok {
		t.Fatalf("revision 78 unexpectedly resolved from cache")
	}
}

// TestHandlePassthrough verifies that requests to Charmhub API paths this
// proxy doesn't cache (e.g. resource revision lookups) are forwarded to
// upstream with status, body and content-type preserved. Without this, an
// unhandled path 404s with a plain-text body, which the Juju client rejects
// with "unexpected charm-hub url ... when parsing headers".
func TestHandlePassthrough(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/charms/resources/juju-qa-test/foo-file/revisions" {
			t.Errorf("unexpected upstream path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"revisions":[]}`))
	}))
	defer upstream.Close()

	s := &server{store: nil, upstream: upstream.URL}

	req := httptest.NewRequest(http.MethodGet, "/v2/charms/resources/juju-qa-test/foo-file/revisions", nil)
	rec := httptest.NewRecorder()
	s.handlePassthrough(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type = %q, want application/json", ct)
	}
	if rec.Body.String() != `{"revisions":[]}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

// TestChannelOnlyNeverCached verifies that channel-only requests (no pinned
// revision) are never served from cache, even after the same charm has been
// resolved and stored under a pinned revision. A channel's resolved revision
// can change at any time, so serving it from cache would return stale data
// once Charmhub advances the channel — this caused a real test failure
// ("expected charm revision to be updated ... but it is still N").
func TestChannelOnlyNeverCached(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	resp := refreshResponse("ubuntu", 77, "abc123", "https://cdn.example/ubuntu_77.charm")
	if err := st.PutRefresh(resp); err != nil {
		t.Fatalf("PutRefresh: %v", err)
	}

	channelOnly := action{Name: "ubuntu", Channel: "latest/stable", Base: "amd64/ubuntu/22.04"}
	if _, ok := st.LookupRefresh(channelOnly); ok {
		t.Fatalf("channel-only request unexpectedly resolved from cache; must always miss to upstream")
	}
}

// TestRefreshByIDOffline verifies the charmrevisioner path: a refresh-by-id
// request (no name, revision in the context) resolves offline via the id
// index populated from a prior response.
func TestRefreshByIDOffline(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// Store a response (as if from a deploy) — it carries the charmhub id.
	resp := refreshResponse("ubuntu", 77, "abc123", "https://cdn.example/ubuntu_77.charm")
	rev := 77
	if err := st.PutRefresh(resp); err != nil {
		t.Fatalf("PutRefresh: %v", err)
	}

	// The charmrevisioner sends refresh-by-id: action has only id, revision in
	// context. The proxy parses this into action{ID, Revision} (name empty).
	workerAction := action{ID: "charm-id-ubuntu", Revision: &rev}
	got, ok := st.LookupRefresh(workerAction)
	if !ok {
		t.Fatalf("refresh-by-id did not resolve from cache")
	}
	if string(got) != string(resp) {
		t.Fatalf("refresh-by-id got different body than stored")
	}

	// A refresh-by-id for a revision we haven't seen must miss.
	rev99 := 99
	if _, ok := st.LookupRefresh(action{ID: "charm-id-ubuntu", Revision: &rev99}); ok {
		t.Fatalf("refresh-by-id for unseen revision unexpectedly resolved")
	}
}

// TestParseRefreshByIDAction verifies that refresh-by-id requests pull
// revision/channel/base from the context array (matched by instance-key),
// since the action itself carries only the id.
func TestParseRefreshByIDAction(t *testing.T) {
	body := []byte(`{
		"context":[{"instance-key":"ik-1","id":"charm-id-ubuntu","revision":77,"tracking-channel":"latest/stable","base":{"architecture":"amd64","name":"ubuntu","channel":"22.04"}}],
		"actions":[{"action":"refresh","instance-key":"ik-1","id":"charm-id-ubuntu"}]
	}`)
	acts, err := parseRefreshActions(body)
	if err != nil {
		t.Fatalf("parse refresh-by-id: %v", err)
	}
	if len(acts) != 1 {
		t.Fatalf("expected 1 action, got %d", len(acts))
	}
	a := acts[0]
	if a.ID != "charm-id-ubuntu" || a.Name != "" {
		t.Fatalf("id/name parsed wrong: %+v", a)
	}
	if a.Revision == nil || *a.Revision != 77 {
		t.Fatalf("revision not pulled from context: %+v", a)
	}
	if a.Channel != "latest/stable" || a.Base != "amd64/ubuntu/22.04" {
		t.Fatalf("channel/base not pulled from context: %+v", a)
	}
}

// TestParseRefreshActions verifies action extraction across the different
// serialisations the provider and controller produce.
func TestParseRefreshActions(t *testing.T) {
	// Provider style: pinned revision, no channel/base.
	p := []byte(`{"context":[],"actions":[{"action":"install","instance-key":"key-0","name":"ubuntu","revision":77}],"fields":["revision"]}`)
	acts, err := parseRefreshActions(p)
	if err != nil {
		t.Fatalf("parse provider request: %v", err)
	}
	if len(acts) != 1 || acts[0].Name != "ubuntu" || acts[0].Revision == nil || *acts[0].Revision != 77 {
		t.Fatalf("provider action parsed wrong: %+v", acts)
	}

	// Controller style: channel + base, no revision.
	c := []byte(`{"actions":[{"action":"install","instance-key":"k","name":"ubuntu","channel":"latest/stable","base":{"architecture":"amd64","name":"ubuntu","channel":"22.04"}}]}`)
	acts, err = parseRefreshActions(c)
	if err != nil {
		t.Fatalf("parse controller request: %v", err)
	}
	if len(acts) != 1 || acts[0].Channel != "latest/stable" || acts[0].Revision != nil || acts[0].Base != "amd64/ubuntu/22.04" {
		t.Fatalf("controller action parsed wrong: %+v", acts)
	}
}

// TestRewriteInstanceKey verifies that the instance-key in a cached response
// is rewritten to match the current request. Without this, the client's
// Ensure() rejects the response with "install action key not valid" because
// the cached response carries a stale per-request UUID.
func TestRewriteInstanceKey(t *testing.T) {
	body := []byte(`{"results":[{"instance-key":"old-uuid","charm":{"name":"ubuntu","revision":77}}]}`)
	rewritten := rewriteInstanceKey(body, "new-uuid")

	var doc struct {
		Results []struct {
			InstanceKey string `json:"instance-key"`
			Charm       struct {
				Name string `json:"name"`
			} `json:"charm"`
		} `json:"results"`
	}
	if err := json.Unmarshal(rewritten, &doc); err != nil {
		t.Fatalf("rewritten body is not valid JSON: %v", err)
	}
	if len(doc.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(doc.Results))
	}
	if doc.Results[0].InstanceKey != "new-uuid" {
		t.Fatalf("instance-key = %q, want %q", doc.Results[0].InstanceKey, "new-uuid")
	}
	if doc.Results[0].Charm.Name != "ubuntu" {
		t.Fatalf("charm name not preserved: %q", doc.Results[0].Charm.Name)
	}
}

// TestRewriteDownloadURLs verifies that:
//   - the download.url in each result is rewritten to an absolute proxy URL,
//   - the real upstream URL is recorded in the store for backfill,
//   - all other fields are preserved verbatim (including unknown ones).
func TestRewriteDownloadURLs(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	const realURL = "https://storage.example.com/charms/ubuntu_77.charm"
	const hash = "abc123def456"
	// Include an unknown field ("future-field") to prove the rewrite is
	// lossless for fields the transport package doesn't model.
	body := []byte(`{
		"results": [
			{
				"charm": {
					"name": "ubuntu",
					"revision": 77,
					"download": {
						"hash-sha-256": "` + hash + `",
						"hash-sha-384": "deadbeef",
						"size": 12345,
						"url": "` + realURL + `"
					},
					"future-field": {"keep": "me"}
				},
				"effective-channel": "latest/stable"
			}
		]
	}`)

	rewritten, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}

	var doc map[string]any
	if err := json.Unmarshal(rewritten, &doc); err != nil {
		t.Fatalf("rewritten body is not valid JSON: %v", err)
	}
	results := doc["results"].([]any)
	result := results[0].(map[string]any)
	entity := result["charm"].(map[string]any)
	download := entity["download"].(map[string]any)

	gotURL := download["url"].(string)
	wantURL := "http://127.0.0.1:8080/blob/" + hash
	if gotURL != wantURL {
		t.Fatalf("download.url = %q, want %q", gotURL, wantURL)
	}

	// Other download fields preserved.
	if download["hash-sha-384"] != "deadbeef" {
		t.Errorf("hash-sha-384 not preserved: %v", download["hash-sha-384"])
	}
	if download["size"] != float64(12345) {
		t.Errorf("size not preserved: %v", download["size"])
	}
	// Unknown field preserved.
	if ff, ok := entity["future-field"].(map[string]any); !ok || ff["keep"] != "me" {
		t.Errorf("future-field not preserved: %v", entity["future-field"])
	}
	// Sibling fields preserved.
	if result["effective-channel"] != "latest/stable" {
		t.Errorf("effective-channel not preserved: %v", result["effective-channel"])
	}

	// The real URL was recorded for backfill.
	recorded, ok := st.BlobURL(hash)
	if !ok {
		t.Fatalf("BlobURL(%q) not recorded", hash)
	}
	if recorded != realURL {
		t.Fatalf("recorded URL = %q, want %q", recorded, realURL)
	}
}

// TestRewriteDownloadURLsNoResults verifies that responses without a results
// array (e.g. error-list responses) pass through unchanged.
func TestRewriteDownloadURLsNoResults(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	body := []byte(`{"error-list":[{"code":"not-found","message":"no such charm"}]}`)
	rewritten, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}
	// The body should be semantically identical (key present, no results key).
	var doc map[string]any
	if err := json.Unmarshal(rewritten, &doc); err != nil {
		t.Fatalf("rewritten body is not valid JSON: %v", err)
	}
	if _, ok := doc["error-list"]; !ok {
		t.Errorf("error-list dropped from response")
	}
}

// TestStorePutRefreshThenRead verifies the refresh store round-trips and is
// persisted to disk (so an actions/cache restore picks it up).
func TestStorePutRefreshThenRead(t *testing.T) {
	dir := t.TempDir()
	st, err := newStore(dir)
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	rev := 77
	a := action{Name: "ubuntu", Revision: &rev}
	body := refreshResponse("ubuntu", 77, "abc123", "https://cdn.example/u.charm")
	if err := st.PutRefresh(body); err != nil {
		t.Fatalf("PutRefresh: %v", err)
	}
	got, ok := st.LookupRefresh(a)
	if !ok {
		t.Fatalf("LookupRefresh miss after PutRefresh")
	}
	if string(got) != string(body) {
		t.Fatalf("LookupRefresh body mismatch: got %q want %q", got, body)
	}
	// Confirm it landed on disk under the canonical path.
	if _, err := os.Stat(filepath.Join(dir, "refresh", canonicalKey("ubuntu", 77)+".json")); err != nil {
		t.Fatalf("refresh file not on disk: %v", err)
	}
}

// TestStoreBlobURLRoundTrip verifies the blob URL index persists.
func TestStoreBlobURLRoundTrip(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}
	if _, ok := st.BlobURL("nope"); ok {
		t.Fatalf("BlobURL returned ok for missing key")
	}
	if err := st.PutBlobURL("abc", "https://cdn.example/blob"); err != nil {
		t.Fatalf("PutBlobURL: %v", err)
	}
	got, ok := st.BlobURL("abc")
	if !ok {
		t.Fatalf("BlobURL miss after PutBlobURL")
	}
	if got != "https://cdn.example/blob" {
		t.Fatalf("got %q, want cdn url", got)
	}
}
