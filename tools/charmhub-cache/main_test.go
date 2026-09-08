// Copyright 2026 Canonical Ltd.
// Licensed under the Apache License, Version 2.0, see LICENCE file for details.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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

// putRefresh stores a response the way handleRefresh does in production:
// extract the resolved identity via rewriteDownloadURLs (the same single
// parse), then PutRefresh. Fails the test if the response doesn't resolve
// to exactly one identity.
func putRefresh(t *testing.T, st *store, body []byte, variant string) {
	t.Helper()
	_, identities, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}
	if len(identities) != 1 {
		t.Fatalf("expected exactly 1 resolved identity, got %d", len(identities))
	}
	if err := st.PutRefresh(body, identities[0], variant); err != nil {
		t.Fatalf("PutRefresh: %v", err)
	}
}

// TestPutRefreshRejectsMultiResult is a regression test: caching a batched
// (multi-result) response under one canonical key would later be replayed
// verbatim to an unrelated single-action request expecting exactly one
// result. The Juju client rejects that with "more than 1 result found"
// (core/charm/repository.(*CharmHubRepository).refreshOne requires len==1).
// handleRefresh must refuse to store multi-result bodies.
func TestPutRefreshRejectsMultiResult(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// The production store path: rewriteDownloadURLs extracts the
	// identities, and handleRefresh only stores when there is exactly one.
	body := multiResultRefreshResponse("ubuntu", "postgresql", 77, 10)
	_, identities, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}
	if len(identities) != 2 {
		t.Fatalf("expected 2 resolved identities from a batched response, got %d", len(identities))
	}

	// Confirm nothing was cached: a later single-action lookup for either
	// charm must miss (go to upstream), not return the batched body.
	rev := 77
	if _, ok := st.LookupRefresh(action{Name: "ubuntu", Revision: &rev}); ok {
		t.Fatalf("multi-result response was cached despite the store gate rejecting it")
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
	putRefresh(t, st, resp, "")

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
	putRefresh(t, st, resp, "")

	channelOnly := action{Name: "ubuntu", Channel: "latest/stable"}
	if _, ok := st.LookupRefresh(channelOnly); ok {
		t.Fatalf("channel-only request unexpectedly resolved from cache; must always miss to upstream")
	}
}

// TestByIDNeverCached verifies that by-id refreshes (the charmrevisioner
// worker, which never fires within a CI job's lifetime) are never served
// from cache: they carry no name, so they fail isCacheable and always pass
// through to upstream.
func TestByIDNeverCached(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	resp := refreshResponse("ubuntu", 77, "abc123", "https://cdn.example/ubuntu_77.charm")
	putRefresh(t, st, resp, "")

	rev := 77
	if _, ok := st.LookupRefresh(action{Revision: &rev}); ok {
		t.Fatalf("by-id request (no name) unexpectedly resolved from cache; must always miss to upstream")
	}
}

// TestParseRefreshActions verifies action extraction across the different
// serialisations the provider and controller produce.
func TestParseRefreshActions(t *testing.T) {
	// Provider style: pinned revision, no channel.
	p := []byte(`{"context":[],"actions":[{"action":"install","instance-key":"key-0","name":"ubuntu","revision":77}],"fields":["revision"]}`)
	acts, err := parseRefreshActions(p)
	if err != nil {
		t.Fatalf("parse provider request: %v", err)
	}
	if len(acts) != 1 || acts[0].Name != "ubuntu" || acts[0].Revision == nil || *acts[0].Revision != 77 {
		t.Fatalf("provider action parsed wrong: %+v", acts)
	}

	// Controller style: channel, no revision (base is ignored by the
	// parser — it doesn't feed the cacheability check or key).
	c := []byte(`{"actions":[{"action":"install","instance-key":"k","name":"ubuntu","channel":"latest/stable","base":{"architecture":"amd64","name":"ubuntu","channel":"22.04"}}]}`)
	acts, err = parseRefreshActions(c)
	if err != nil {
		t.Fatalf("parse controller request: %v", err)
	}
	if len(acts) != 1 || acts[0].Channel != "latest/stable" || acts[0].Revision != nil {
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

	rewritten, identities, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}

	// The resolved identity is extracted from the same single parse.
	if len(identities) != 1 || identities[0].name != "ubuntu" || identities[0].revision != 77 {
		t.Errorf("resolved identity = %+v, want ubuntu@77", identities)
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
	rewritten, identities, err := rewriteDownloadURLs(body, st, "http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("rewriteDownloadURLs: %v", err)
	}
	// No results means no identity: such responses are never cacheable.
	if len(identities) != 0 {
		t.Errorf("expected no identities from an error-list response, got %d", len(identities))
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
	putRefresh(t, st, body, "")
	got, ok := st.LookupRefresh(a)
	if !ok {
		t.Fatalf("LookupRefresh miss after PutRefresh")
	}
	if string(got) != string(body) {
		t.Fatalf("LookupRefresh body mismatch: got %q want %q", got, body)
	}
	// Confirm it landed on disk under the canonical path.
	if _, err := os.Stat(filepath.Join(dir, "refresh", canonicalKey("ubuntu", 77, "")+".json")); err != nil {
		t.Fatalf("refresh file not on disk: %v", err)
	}
}

// TestFieldsMismatchNotServedFromCache is a regression test for a real CI
// failure: the provider's ActionExists requests "actions-yaml" (needed to
// check for a defined action) while the controller's own deploy-time resolve
// does not. If a response cached for one field set were replayed to a
// request needing a different field set, the response could be missing
// fields the caller depends on (e.g. actions-yaml), causing ActionExists to
// incorrectly report the action as undefined. The cache must key on the
// requested fields so these never conflate.
func TestFieldsMismatchNotServedFromCache(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// Controller-style resolve: no actions-yaml in the response, cached
	// under the controller's field set (via the same variantKey lookups use).
	resp := refreshResponse("juju-qa-dummy-source", 6, "abc123", "https://cdn.example/dummy_6.charm")
	rev := 6
	const controllerFields = "bases,config-yaml,download,id,license,metadata-yaml,name,publisher,resources,revision,summary,type,version"
	controllerAction := action{Name: "juju-qa-dummy-source", Revision: &rev, FieldsKey: controllerFields}
	putRefresh(t, st, resp, variantKey(controllerAction))

	// Provider's ActionExists requests a different field set (includes
	// actions-yaml). It must NOT hit the controller's cache entry.
	const providerFields = "actions-yaml,bases,metadata-yaml,name,resources,revision"
	providerAction := action{Name: "juju-qa-dummy-source", Revision: &rev, FieldsKey: providerFields}
	if _, ok := st.LookupRefresh(providerAction); ok {
		t.Fatalf("provider request with different fields incorrectly hit the controller's cache entry")
	}

	// A request with the SAME field set as what was cached still hits.
	if _, ok := st.LookupRefresh(controllerAction); !ok {
		t.Fatalf("request with matching fields unexpectedly missed cache")
	}
}

// TestRevisionWithChannelNeverCached is a regression test for a real CI
// failure: RefreshOne (juju3/charmhub/refresh.go) sends BOTH the currently
// installed revision AND the tracking channel, to ask "has this channel
// advanced past what I have installed?". That answer changes as the channel
// advances, exactly like a channel-only resolve. The proxy previously only
// treated a nil Revision as "never cache", so this shape (revision set,
// channel also set) was wrongly served from cache: after switching to a new
// channel at the same starting revision, the update check kept replaying
// the stale "no update available" response recorded under the old channel.
func TestRevisionWithChannelNeverCached(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	resp := refreshResponse("ubuntu", 25, "abc123", "https://cdn.example/ubuntu_25.charm")
	putRefresh(t, st, resp, "")

	rev := 25
	updateCheck := action{Name: "ubuntu", Revision: &rev, Channel: "2.0/stable"}
	if _, ok := st.LookupRefresh(updateCheck); ok {
		t.Fatalf("revision+channel request unexpectedly resolved from cache; must always miss to upstream")
	}
}

// TestUpdateCheckDoesNotPoisonPinnedRevisionCache is an end-to-end
// regression test for a real CI failure: LookupRefresh correctly refuses to
// SERVE a revision+channel ("is there an update?") response from cache, but
// handleRefresh still unconditionally STORED every single-action cache miss
// — including that same revision+channel shape. Since it always misses on
// lookup, it always falls through to upstream and gets written to the store
// under the SAME canonical key a later plain pinned-revision request uses.
// The update-check response reflects the CURRENT channel head (e.g.
// embedded resource revisions), so it poisoned the entry for a later plain
// resolve of that same revision: coredns@191's deploy-time resolve was
// served a resources array reflecting whatever coredns-image the channel
// currently points to (70) instead of resolving fresh, producing the wrong
// resource revision deterministically on every run.
func TestUpdateCheckDoesNotPoisonPinnedRevisionCache(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// The upstream always resolves coredns@191 with whatever the CURRENT
	// channel-head image resource revision is (70), simulating Charmhub's
	// real behaviour: the charm revision is pinned, but embedded resource
	// revisions still reflect the channel's current state.
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"charm":{"id":"charm-id-coredns","name":"coredns","revision":191,` +
			`"resources":[{"name":"coredns-image","revision":70}]}}]}`))
	}))
	defer upstream.Close()

	s := &server{store: st, upstream: upstream.URL}

	// Step 1: an "is there an update?" check — revision AND channel both
	// set, as RefreshOne sends. This must miss the (empty) cache, go
	// upstream, and — critically — must NOT be stored afterwards.
	updateCheckBody := []byte(`{
		"context":[{"instance-key":"ik-1","id":"charm-id-coredns","revision":191,"tracking-channel":"latest/stable"}],
		"actions":[{"action":"refresh","instance-key":"ik-1","id":"charm-id-coredns"}]
	}`)
	req := httptest.NewRequest(http.MethodPost, refreshPath, bytes.NewReader(updateCheckBody))
	rec := httptest.NewRecorder()
	s.handleRefresh(rec, req)
	if rec.Header().Get("X-Charmhub-Cache") != "MISS" {
		t.Fatalf("update-check: X-Charmhub-Cache = %q, want MISS", rec.Header().Get("X-Charmhub-Cache"))
	}

	// Step 2: a plain pinned-revision resolve for the SAME revision, no
	// channel — the deploy's actual charm resolve. It must resolve fresh
	// (cache miss), not be served the update-check's poisoned response.
	pinnedBody := []byte(`{"actions":[{"action":"install","instance-key":"ik-2","name":"coredns","revision":191}]}`)
	req2 := httptest.NewRequest(http.MethodPost, refreshPath, bytes.NewReader(pinnedBody))
	rec2 := httptest.NewRecorder()
	s.handleRefresh(rec2, req2)
	if rec2.Header().Get("X-Charmhub-Cache") == "HIT" {
		t.Fatalf("pinned-revision resolve was served from cache; the update-check response poisoned the entry")
	}
}

// TestResourceRevisionRequestsNotConflated is the end-to-end regression test
// for the real CI failure in TestAcc_ResourceRevisionUpdatesMicrok8s
// (coredns-image "70" -> "59"). To resolve a SPECIFIC k8s image resource
// revision, the controller (juju3 core/charm/repository.configsByName ->
// charmhub.AddResource) sends a refresh with revision pinned (191, no
// channel) plus a "resource-revisions" discriminator naming the wanted
// resource revision. Two such requests for the SAME charm+revision but
// DIFFERENT resource revisions (59 vs 70) differ ONLY in resource-revisions.
// These ARE cached (folded into the cache key via variantKey), so this test
// asserts correct partitioning: distinct resource revisions get distinct
// entries (never conflated), while a repeat of the SAME resource revision is
// served the correct cached body. Conflating them was why the test failed on
// the proxy branch but not on main (live Charmhub honours each request).
func TestResourceRevisionRequestsNotConflated(t *testing.T) {
	st, err := newStore(t.TempDir())
	if err != nil {
		t.Fatalf("newStore: %v", err)
	}

	// Upstream echoes back the requested resource revision so we can tell
	// which request a response actually came from, and counts hits so we can
	// confirm the repeat is served from cache (not re-fetched).
	var upstreamHits int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits++
		body, _ := io.ReadAll(r.Body)
		rev := 70
		if bytes.Contains(body, []byte(`"revision":59`)) {
			rev = 59
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, `{"results":[{"charm":{"id":"charm-id-coredns","name":"coredns","revision":191,`+
			`"resources":[{"name":"coredns-image","revision":%d}]}}]}`, rev)
	}))
	defer upstream.Close()

	s := &server{store: st, upstream: upstream.URL}

	resReq := func(resRev int) *httptest.ResponseRecorder {
		body := []byte(fmt.Sprintf(`{"actions":[{"action":"install","instance-key":"ik","name":"coredns","revision":191,`+
			`"resource-revisions":[{"name":"coredns-image","revision":%d}]}]}`, resRev))
		req := httptest.NewRequest(http.MethodPost, refreshPath, bytes.NewReader(body))
		rec := httptest.NewRecorder()
		s.handleRefresh(rec, req)
		return rec
	}

	// Resolve resource rev 70 first (populates the cache), then rev 59.
	if rec := resReq(70); rec.Header().Get("X-Charmhub-Cache") != "MISS" {
		t.Fatalf("first rev-70 request: X-Charmhub-Cache = %q, want MISS", rec.Header().Get("X-Charmhub-Cache"))
	}
	rec := resReq(59)

	// The rev-59 request must NOT be served the rev-70 cached response: it
	// must resolve fresh and return the rev-59 body.
	if rec.Header().Get("X-Charmhub-Cache") == "HIT" {
		t.Fatalf("rev-59 request was served the rev-70 cache entry (resource revisions conflated)")
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"revision":59`)) {
		t.Fatalf("resource rev 59 request got wrong response body: %s", rec.Body.String())
	}
	if upstreamHits != 2 {
		t.Fatalf("expected both distinct resource-revision requests to reach upstream, got %d hits", upstreamHits)
	}

	// A REPEAT of rev 70 must now be served from cache (proving resource
	// revisions are cached, not skipped) and return the rev-70 body.
	repeat := resReq(70)
	if repeat.Header().Get("X-Charmhub-Cache") != "HIT" {
		t.Fatalf("repeated rev-70 request: X-Charmhub-Cache = %q, want HIT", repeat.Header().Get("X-Charmhub-Cache"))
	}
	if !bytes.Contains(repeat.Body.Bytes(), []byte(`"revision":70`)) {
		t.Fatalf("repeated rev-70 request got wrong body: %s", repeat.Body.String())
	}
	if upstreamHits != 2 {
		t.Fatalf("repeat should have been served from cache, but upstream was hit again (%d)", upstreamHits)
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
