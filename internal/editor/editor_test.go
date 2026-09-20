package editor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/outsideris/retrotech/internal/editor/assist"
)

// newTestServer starts the editor over a temp repo seeded with the directories
// the editor needs (content/episodes for storage, public/ for preview assets).
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, "content", "episodes"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "public"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "public", "styles.css"), []byte("body{}"), 0644); err != nil {
		t.Fatal(err)
	}

	ed, err := New(Config{RepoDir: repo})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv := httptest.NewServer(ed.Handler())
	t.Cleanup(srv.Close)
	return srv, repo
}

func do(t *testing.T, srv *httptest.Server, method, path string, body any) (*http.Response, []byte) {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, srv.URL+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp, data
}

func mustStatus(t *testing.T, resp *http.Response, want int) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("status: got %d, want %d", resp.StatusCode, want)
	}
}

func TestNewRejectsRepoWithoutEpisodes(t *testing.T) {
	if _, err := New(Config{RepoDir: t.TempDir()}); err == nil {
		t.Fatal("expected New to reject a repo with no content/episodes")
	}
}

func TestAPIEpisodeLifecycle(t *testing.T) {
	srv, repo := newTestServer(t)

	resp, body := do(t, srv, "GET", "/_write/api/episodes", nil)
	mustStatus(t, resp, http.StatusOK)
	if strings.TrimSpace(string(body)) != "[]" {
		t.Errorf("expected empty list, got %s", body)
	}

	form := EpisodeForm{
		ID: "9z", Title: "Test Episode\n", Date: "2026/06/21",
		Description:   "설명\n",
		EnclosureURL:  "https://retrotech-episodes.outsider.dev/9z.mp3",
		EnclosureSize: 1000, Duration: "10:00",
		Structured: true, Intro: "intro",
		References: []Reference{{Text: "Go", URL: "https://go.dev"}},
	}

	resp, _ = do(t, srv, "POST", "/_write/api/episodes", form)
	mustStatus(t, resp, http.StatusCreated)
	if _, err := os.Stat(filepath.Join(repo, "content", "episodes", "9z.md")); err != nil {
		t.Fatalf("episode file not written: %v", err)
	}

	resp, _ = do(t, srv, "POST", "/_write/api/episodes", form)
	mustStatus(t, resp, http.StatusConflict)

	resp, body = do(t, srv, "GET", "/_write/api/episodes/9z", nil)
	mustStatus(t, resp, http.StatusOK)
	var got EpisodeForm
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if got.Title != "Test Episode\n" {
		t.Errorf("title: got %q", got.Title)
	}
	if len(got.References) != 1 || got.References[0].URL != "https://go.dev" {
		t.Errorf("references: %#v", got.References)
	}

	resp, body = do(t, srv, "GET", "/_write/api/episodes", nil)
	mustStatus(t, resp, http.StatusOK)
	var list []EpisodeSummary
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "9z" || list[0].Duration != "10:00" {
		t.Errorf("list: %#v", list)
	}

	form.Duration = "20:00"
	resp, _ = do(t, srv, "PUT", "/_write/api/episodes/9z", form)
	mustStatus(t, resp, http.StatusOK)
	resp, body = do(t, srv, "GET", "/_write/api/episodes/9z", nil)
	json.Unmarshal(body, &got)
	if got.Duration != "20:00" {
		t.Errorf("after update, duration = %q", got.Duration)
	}

	resp, _ = do(t, srv, "DELETE", "/_write/api/episodes/9z", nil)
	mustStatus(t, resp, http.StatusNoContent)
	resp, _ = do(t, srv, "GET", "/_write/api/episodes/9z", nil)
	mustStatus(t, resp, http.StatusNotFound)
}

// TestAPIDerivedDescription: over HTTP, a structured episode's description
// follows the intro (links stripped) regardless of what the client sends — the
// UI keeps description in a hidden, possibly stale field, so the server must
// never trust it.
func TestAPIDerivedDescription(t *testing.T) {
	srv, _ := newTestServer(t)
	form := EpisodeForm{
		ID: "9z", Title: "T\n", Date: "2026/08/02",
		Description:  "클라이언트가 보낸 값",
		EnclosureURL: "https://retrotech-episodes.outsider.dev/9z.mp3",
		Duration:     "10:00",
		Structured:   true, Intro: "[Go](https://go.dev) 이야기.",
	}

	resp, body := do(t, srv, "POST", "/_write/api/episodes", form)
	mustStatus(t, resp, http.StatusCreated)
	var got EpisodeForm
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}
	if want := "Go 이야기.\n"; got.Description != want {
		t.Errorf("create response description = %q, want %q", got.Description, want)
	}

	// Intro edit → description follows, in the response and on re-read.
	form.Intro = "새로운 도입부."
	resp, body = do(t, srv, "PUT", "/_write/api/episodes/9z", form)
	mustStatus(t, resp, http.StatusOK)
	json.Unmarshal(body, &got)
	if want := "새로운 도입부.\n"; got.Description != want {
		t.Errorf("update response description = %q, want %q", got.Description, want)
	}

	// Unchanged intro + stale client description → stored description kept.
	form.Description = "stale hidden field"
	resp, _ = do(t, srv, "PUT", "/_write/api/episodes/9z", form)
	mustStatus(t, resp, http.StatusOK)
	resp, body = do(t, srv, "GET", "/_write/api/episodes/9z", nil)
	mustStatus(t, resp, http.StatusOK)
	json.Unmarshal(body, &got)
	if want := "새로운 도입부.\n"; got.Description != want {
		t.Errorf("stale client description leaked through: %q, want %q", got.Description, want)
	}
}

func TestAPIErrorCodes(t *testing.T) {
	srv, _ := newTestServer(t)

	// Valid slug, but no such episode.
	resp, _ := do(t, srv, "GET", "/_write/api/episodes/zz", nil)
	mustStatus(t, resp, http.StatusNotFound)

	// Invalid slug (dot is not allowed).
	resp, _ = do(t, srv, "GET", "/_write/api/episodes/a.b", nil)
	mustStatus(t, resp, http.StatusBadRequest)

	// Malformed JSON body.
	req, _ := http.NewRequest("POST", srv.URL+"/_write/api/episodes", strings.NewReader("{not json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	mustStatus(t, resp, http.StatusBadRequest)
}

func TestAPIPreviewRendersEpisode(t *testing.T) {
	srv, _ := newTestServer(t)
	form := EpisodeForm{
		ID: "2h", Title: "Preview Title", Date: "2026/06/21",
		Description: "desc", Duration: "5:00",
		Structured: true, Intro: "hello world",
		References: []Reference{{Text: "Go", URL: "https://go.dev"}},
	}
	resp, body := do(t, srv, "POST", "/_write/api/preview", form)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type: %s", ct)
	}
	html := string(body)
	if !strings.Contains(html, "Preview Title") {
		t.Errorf("preview missing title")
	}
	if !strings.Contains(html, `href="https://go.dev"`) {
		t.Errorf("preview missing reference link")
	}
}

func TestServesUIAndPublicAssets(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, body := do(t, srv, "GET", "/_write/", nil)
	mustStatus(t, resp, http.StatusOK)
	if !strings.Contains(string(body), "<title>RetroTech") {
		t.Errorf("index.html not served")
	}

	resp, _ = do(t, srv, "GET", "/_write/app.js", nil)
	mustStatus(t, resp, http.StatusOK)
	// Embedded UI must be uncacheable so app updates aren't masked by a stale
	// cache on the fixed loopback origin.
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("app.js Cache-Control = %q, want no-store", cc)
	}

	// public/ asset, used by preview pages.
	resp, _ = do(t, srv, "GET", "/styles.css", nil)
	mustStatus(t, resp, http.StatusOK)

	// "/" redirects to the editor (no-follow client to observe the 302).
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(srv.URL + "/")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/_write/" {
		t.Errorf("root: status %d, location %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestDraftAPILifecycle(t *testing.T) {
	srv, repo := newTestServer(t)

	resp, body := do(t, srv, "POST", "/_write/api/drafts", nil)
	mustStatus(t, resp, http.StatusCreated)
	var created struct {
		Slug string      `json:"slug"`
		Form EpisodeForm `json:"form"`
	}
	if err := json.Unmarshal(body, &created); err != nil {
		t.Fatal(err)
	}
	if created.Slug == "" || !created.Form.Structured {
		t.Fatalf("create response: %s", body)
	}
	slug := created.Slug
	if _, err := os.Stat(filepath.Join(repo, "content", "drafts", slug+".json")); err != nil {
		t.Fatalf("draft json not written: %v", err)
	}

	resp, body = do(t, srv, "GET", "/_write/api/drafts", nil)
	mustStatus(t, resp, http.StatusOK)
	var list []DraftSummary
	json.Unmarshal(body, &list)
	if len(list) != 1 || list[0].Slug != slug {
		t.Errorf("draft list: %#v", list)
	}

	f := created.Form
	f.ID, f.Title = "2h", "Draft Title"
	f.EnclosureURL, f.Duration = "https://retrotech-episodes.outsider.dev/2h.mp3", "5:00"
	resp, _ = do(t, srv, "PUT", "/_write/api/drafts/"+slug, f)
	mustStatus(t, resp, http.StatusOK)

	resp, body = do(t, srv, "GET", "/_write/api/drafts/"+slug, nil)
	mustStatus(t, resp, http.StatusOK)
	var gf EpisodeForm
	json.Unmarshal(body, &gf)
	if gf.ID != "2h" || gf.Title != "Draft Title" {
		t.Errorf("draft get: %#v", gf)
	}

	resp, body = do(t, srv, "POST", "/_write/api/drafts/"+slug+"/publish", nil)
	mustStatus(t, resp, http.StatusOK)
	var pub map[string]string
	json.Unmarshal(body, &pub)
	if pub["id"] != "2h" {
		t.Errorf("publish response: %s", body)
	}

	// Episode now exists; the draft is gone.
	resp, _ = do(t, srv, "GET", "/_write/api/episodes/2h", nil)
	mustStatus(t, resp, http.StatusOK)
	resp, _ = do(t, srv, "GET", "/_write/api/drafts/"+slug, nil)
	mustStatus(t, resp, http.StatusNotFound)
}

// TestDraftCreateFindOrCreateByID: POST /drafts with {"id"} returns the
// existing draft for that episode id (200) instead of creating another (201) —
// the guard that keeps repeated script imports from piling up drafts.
func TestDraftCreateFindOrCreateByID(t *testing.T) {
	srv, _ := newTestServer(t)

	// Plain create (no body), then stamp the draft with an episode id.
	resp, body := do(t, srv, "POST", "/_write/api/drafts", nil)
	mustStatus(t, resp, http.StatusCreated)
	var created struct {
		Slug string      `json:"slug"`
		Form EpisodeForm `json:"form"`
	}
	json.Unmarshal(body, &created)
	f := created.Form
	f.ID = "2h"
	resp, _ = do(t, srv, "PUT", "/_write/api/drafts/"+created.Slug, f)
	mustStatus(t, resp, http.StatusOK)

	// Create with the same id → the existing draft comes back, no new one.
	resp, body = do(t, srv, "POST", "/_write/api/drafts", map[string]string{"id": "2h"})
	mustStatus(t, resp, http.StatusOK)
	var reused struct {
		Slug string `json:"slug"`
	}
	json.Unmarshal(body, &reused)
	if reused.Slug != created.Slug {
		t.Errorf("want reused slug %q, got %q", created.Slug, reused.Slug)
	}
	resp, body = do(t, srv, "GET", "/_write/api/drafts", nil)
	mustStatus(t, resp, http.StatusOK)
	var list []DraftSummary
	json.Unmarshal(body, &list)
	if len(list) != 1 {
		t.Errorf("draft count = %d, want 1 (no pile-up)", len(list))
	}

	// A different id still creates a fresh draft.
	resp, _ = do(t, srv, "POST", "/_write/api/drafts", map[string]string{"id": "9z"})
	mustStatus(t, resp, http.StatusCreated)
}

func TestDraftPublishWithoutIDIsRejected(t *testing.T) {
	srv, _ := newTestServer(t)
	_, body := do(t, srv, "POST", "/_write/api/drafts", nil)
	var created struct {
		Slug string `json:"slug"`
	}
	json.Unmarshal(body, &created)

	// New draft has no episode id yet → publish is a 400, draft survives.
	resp, _ := do(t, srv, "POST", "/_write/api/drafts/"+created.Slug+"/publish", nil)
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "GET", "/_write/api/drafts/"+created.Slug, nil)
	mustStatus(t, resp, http.StatusOK)
}

// TestWriteAssistError: a CLI killed by the timeout must surface as a clear
// 504 message, not exec's raw "signal: killed"; other failures keep their
// existing status mapping.
func TestWriteAssistError(t *testing.T) {
	deadline, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-deadline.Done() // ensure the deadline has fired

	rec := httptest.NewRecorder()
	writeAssistError(rec, deadline, errors.New("claude: signal: killed"), 10*time.Minute)
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("timeout status = %d, want 504", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "제한 시간(10분)") || strings.Contains(body, "signal: killed") {
		t.Errorf("timeout body = %s", body)
	}

	rec = httptest.NewRecorder()
	writeAssistError(rec, context.Background(), fmt.Errorf("claude: %w", assist.ErrUnavailable), time.Minute)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("unavailable status = %d, want 503", rec.Code)
	}

	rec = httptest.NewRecorder()
	writeAssistError(rec, context.Background(), errors.New("boom"), time.Minute)
	if rec.Code != http.StatusBadGateway || !strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("generic: status %d body %s", rec.Code, rec.Body.String())
	}
}

// The sidebar fills its model/effort dropdowns from this payload, so it must
// carry each CLI's models and the levels each model takes.
// app.js hides the effort knob through this id when the chosen model takes no
// level. Renaming it in one file and not the other throws at load and leaves
// the sidebar dead, which no Go test would otherwise notice.
func TestAssistEffortKnobIdIsWiredUp(t *testing.T) {
	html, err := assetsFS.ReadFile("assets/index.html")
	if err != nil {
		t.Fatal(err)
	}
	js, err := assetsFS.ReadFile("assets/app.js")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(html), `id="assist-effort-knob"`) {
		t.Error("index.html lost the effort knob id app.js looks up")
	}
	if !strings.Contains(string(js), `$("assist-effort-knob")`) {
		t.Error("app.js no longer looks up the effort knob id")
	}
}

func TestAssistProvidersEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, body := do(t, srv, "GET", "/_write/api/assist/providers", nil)
	mustStatus(t, resp, http.StatusOK)
	var list []struct {
		Name      string `json:"name"`
		Available bool   `json:"available"`
		Models    []struct {
			ID      string   `json:"id"`
			Label   string   `json:"label"`
			Efforts []string `json:"efforts"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, p := range list {
		byName[p.Name] = i
	}
	for _, n := range []string{"claude", "codex", "gemini"} {
		if _, ok := byName[n]; !ok {
			t.Fatalf("providers missing %q: %s", n, body)
		}
	}
	for _, n := range []string{"claude", "codex"} {
		models := list[byName[n]].Models
		if len(models) == 0 {
			t.Fatalf("%s offers no models: %s", n, body)
		}
		for _, m := range models {
			if m.ID == "" || m.Label == "" {
				t.Errorf("%s model needs an id and a label: %+v", n, m)
			}
		}
		// The response must state each model's levels; at least one model has some.
		levelled := false
		for _, m := range models {
			levelled = levelled || len(m.Efforts) > 0
		}
		if !levelled {
			t.Errorf("%s reports no effort levels at all: %s", n, body)
		}
	}
	// A CLI with no knobs serializes as an empty list, never null — the UI
	// hides its tuning row on length, not on a missing field.
	if models := list[byName["gemini"]].Models; models == nil || len(models) != 0 {
		t.Errorf("gemini models = %v, want []", models)
	}
	if !strings.Contains(string(body), `"models":[]`) {
		t.Errorf("gemini's models should serialize as []: %s", body)
	}
}

// stubAssistCLIs puts fake claude/codex/gemini executables on PATH and returns
// a check that fails if any of them ran. Validation tests must reject before
// exec: without this, a regression would quietly spend real CLI quota instead
// of failing. The marker is written with a shell redirection because PATH no
// longer resolves external commands like touch.
func stubAssistCLIs(t *testing.T) func() {
	t.Helper()
	binDir, ran := t.TempDir(), filepath.Join(t.TempDir(), "ran")
	for _, name := range []string{"claude", "codex", "gemini"} {
		script := "#!/bin/sh\n: > " + ran + "\nprintf 'unexpected run'\n"
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", binDir)
	return func() {
		t.Helper()
		if _, err := os.Stat(ran); err == nil {
			t.Error("an AI CLI was executed; these requests must be rejected first")
		}
	}
}

func TestAssistRunValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	noCLIRan := stubAssistCLIs(t)
	defer noCLIRan()
	// Unknown provider and empty prompt are rejected before any CLI runs.
	resp, _ := do(t, srv, "POST", "/_write/api/assist/run", map[string]string{"provider": "nope", "prompt": "hi"})
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "POST", "/_write/api/assist/run", map[string]string{"provider": "claude", "prompt": "   "})
	mustStatus(t, resp, http.StatusBadRequest)
	// A model or effort outside the catalog is a request error here, not a CLI
	// failure — the answer is the same whether or not that CLI is installed.
	for _, body := range []map[string]string{
		{"provider": "claude", "prompt": "hi", "model": "opus"},
		{"provider": "claude", "prompt": "hi", "model": "claude-opus-5", "effort": "ultra"},
		{"provider": "codex", "prompt": "hi", "model": "gpt-5-codex"},
		{"provider": "codex", "prompt": "hi", "model": "gpt-5.5", "effort": "max"},
	} {
		resp, _ := do(t, srv, "POST", "/_write/api/assist/run", body)
		mustStatus(t, resp, http.StatusBadRequest)
	}
}

func TestAssistAnalyzeValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	noCLIRan := stubAssistCLIs(t)
	defer noCLIRan()
	// Empty script and unknown provider are rejected before any CLI runs.
	resp, _ := do(t, srv, "POST", "/_write/api/assist/analyze", map[string]string{"provider": "claude", "script": "   "})
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "POST", "/_write/api/assist/analyze", map[string]string{"provider": "nope", "script": "# Title"})
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "POST", "/_write/api/assist/analyze", map[string]string{"provider": "codex", "script": "# Title", "model": "gpt-5-codex"})
	mustStatus(t, resp, http.StatusBadRequest)
}
