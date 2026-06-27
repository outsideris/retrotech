package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

func TestAssistProvidersEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, body := do(t, srv, "GET", "/_write/api/assist/providers", nil)
	mustStatus(t, resp, http.StatusOK)
	var list []struct {
		Name      string `json:"name"`
		Available bool   `json:"available"`
	}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, p := range list {
		names[p.Name] = true
	}
	for _, n := range []string{"claude", "codex", "gemini"} {
		if !names[n] {
			t.Errorf("providers missing %q: %s", n, body)
		}
	}
}

func TestAssistRunValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	// Unknown provider and empty prompt are rejected before any CLI runs.
	resp, _ := do(t, srv, "POST", "/_write/api/assist/run", map[string]string{"provider": "nope", "prompt": "hi"})
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "POST", "/_write/api/assist/run", map[string]string{"provider": "claude", "prompt": "   "})
	mustStatus(t, resp, http.StatusBadRequest)
}

func TestAssistAnalyzeValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	// Empty script and unknown provider are rejected before any CLI runs.
	resp, _ := do(t, srv, "POST", "/_write/api/assist/analyze", map[string]string{"provider": "claude", "script": "   "})
	mustStatus(t, resp, http.StatusBadRequest)
	resp, _ = do(t, srv, "POST", "/_write/api/assist/analyze", map[string]string{"provider": "nope", "script": "# Title"})
	mustStatus(t, resp, http.StatusBadRequest)
}
