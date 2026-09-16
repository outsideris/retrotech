package research

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const testOrigin = "http://127.0.0.1:49327"

func fixture(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	skill := t.TempDir()
	os.Mkdir(filepath.Join(skill, "references"), 0755)
	for path, content := range map[string]string{filepath.Join(dir, "index.html"): `<!doctype html><html><head><title>Story</title></head><body><article class="prose"><p>Opening.</p><section id="chapter-01"><h2>One</h2><p>Original one.</p><p>Original two.</p></section></article></body></html>`, filepath.Join(dir, "research.md"): "Opening.\n\n## One\n\nOriginal one.\n\nOriginal two.\n", filepath.Join(skill, "SKILL.md"): "---\nname: test\ndescription: Test skill\n---\n\nOriginal guidance.\n"} {
		if e := os.WriteFile(path, []byte(content), 0644); e != nil {
			t.Fatal(e)
		}
	}
	s, e := NewStore(dir, skill, testOrigin)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func result() *Result {
	return &Result{Title: "A follow-up", Answer: "Summary", Status: "verified", Paragraphs: []Paragraph{{Text: "The evidence <script>alert(1)</script> is explicit.", SourceIDs: []string{"S1"}}}, Sources: []Source{{ID: "S1", Title: "Original record", URL: "https://example.org/primary?x=1&y=2", Author: "A", Date: "2005-01-01", Locator: "Second paragraph", Supports: "Exact supported claim", Kind: "contemporary primary source"}}, Learning: Learning{Gap: "The original story lacked migration costs.", Rule: "Check migration costs when a new tool replaces an earlier one."}}
}
func completed(s *Store) *Job {
	j := &Job{ID: "abc123", Request: Request{AnchorID: s.Anchors[1].ID, Question: "What happened?", Provider: "codex", Model: "gpt-5.6-sol", Effort: "high"}, ChapterID: s.Anchors[1].ChapterID, CreatedAt: now(), UpdatedAt: now(), Status: "completed", Result: result()}
	s.State.Jobs = append(s.State.Jobs, j)
	return j
}
func TestAnchoredPersistenceAndVisibility(t *testing.T) {
	s := fixture(t)
	if len(s.Anchors) != 3 || s.Anchors[1].ChapterID != "chapter-01" {
		t.Fatal(s.Anchors)
	}
	j := completed(s)
	j.Result.Correction = true
	if e := s.saveLocked(true); e != nil {
		t.Fatal(e)
	}
	h, _ := os.ReadFile(filepath.Join(s.Dir, "index.html"))
	m, _ := os.ReadFile(filepath.Join(s.Dir, "research.md"))
	if !strings.Contains(string(h), `Original one.</p><aside class="research-addition"`) || !strings.Contains(string(h), "기존 내용 정정") || bytes.Contains(h, []byte("<script>alert(1)</script>")) {
		t.Fatal("position/correction/escaping failed")
	}
	if !(bytes.Index(m, []byte("Original one.")) < bytes.Index(m, []byte("> **추가 조사")) && bytes.Index(m, []byte("> **추가 조사")) < bytes.Index(m, []byte("Original two."))) {
		t.Fatal("Markdown anchor failed")
	}
	restored, e := NewStore(s.Dir, s.SkillDir, testOrigin)
	if e != nil {
		t.Fatal(e)
	}
	if len(restored.State.Jobs) != 1 {
		t.Fatal("lost history")
	}
	restored.State.Jobs[0].Hidden = true
	if e = restored.saveLocked(true); e != nil {
		t.Fatal(e)
	}
	m, _ = os.ReadFile(filepath.Join(s.Dir, "research.md"))
	if !bytes.Equal(m, s.baseMD) {
		t.Fatal("hiding did not restore original Markdown")
	}
	restored.State.Jobs[0].Hidden = false
	if e = restored.saveLocked(true); e != nil {
		t.Fatal(e)
	}
}
func TestExternalEditAndStateWriteFailure(t *testing.T) {
	t.Run("external", func(t *testing.T) {
		s := fixture(t)
		path := filepath.Join(s.Dir, "research.md")
		os.WriteFile(path, []byte("External edit"), 0644)
		completed(s)
		if !errors.Is(s.saveLocked(true), ErrConflict) {
			t.Fatal("external edit overwritten")
		}
		b, _ := os.ReadFile(path)
		if string(b) != "External edit" {
			t.Fatal("lost external edit")
		}
	})
	t.Run("state failure rolls back pair", func(t *testing.T) {
		s := fixture(t)
		h, _ := os.ReadFile(filepath.Join(s.Dir, "index.html"))
		m, _ := os.ReadFile(filepath.Join(s.Dir, "research.md"))
		path := filepath.Join(s.Dir, ".research/state.json")
		os.Remove(path)
		os.Mkdir(path, 0700)
		completed(s)
		if s.saveLocked(true) == nil {
			t.Fatal("expected write failure")
		}
		gotH, _ := os.ReadFile(filepath.Join(s.Dir, "index.html"))
		gotM, _ := os.ReadFile(filepath.Join(s.Dir, "research.md"))
		if !bytes.Equal(h, gotH) || !bytes.Equal(m, gotM) {
			t.Fatal("pair not rolled back")
		}
	})
}
func TestResultValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Result)
	}{{"missing source", func(r *Result) { r.Paragraphs[0].SourceIDs = []string{"unknown"} }}, {"script URL", func(r *Result) { r.Sources[0].URL = "javascript:alert(1)" }}, {"private URL", func(r *Result) { r.Sources[0].URL = "http://127.0.0.1/a" }}, {"source ID", func(r *Result) { r.Sources[0].ID = `x\" onload=x` }}, {"unexplained uncertainty", func(r *Result) { r.Status = "partial" }}, {"uncited verified", func(r *Result) { r.Paragraphs[0].SourceIDs = nil }}} {
		t.Run(tc.name, func(t *testing.T) {
			r := result()
			tc.change(r)
			if validateResult(r) == nil {
				t.Fatal("accepted invalid result")
			}
		})
	}
	b, _ := json.Marshal(result())
	if _, e := DecodeResult(string(b)); e != nil {
		t.Fatal(e)
	}
	if _, e := DecodeResult(string(b) + " {}"); e == nil {
		t.Fatal("accepted trailing JSON")
	}
}
func request(app *Server, method, path, body, origin string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, testOrigin+path, strings.NewReader(body))
	r.Header.Set("Origin", origin)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("X-RetroTech-Request", "1")
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	return w
}
func awaitJob(t *testing.T, s *Store, id string, status string) {
	t.Helper()
	until := time.Now().Add(3 * time.Second)
	for time.Now().Before(until) {
		_, st := s.Snapshot()
		for _, j := range st.Jobs {
			if j.ID == id && j.Status == status {
				return
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	_, st := s.Snapshot()
	t.Fatalf("job never became %s: %+v", status, st.Jobs[0])
}
func TestHTTPJobsCancellationAndSecurity(t *testing.T) {
	s := fixture(t)
	gate := make(chan bool, 1)
	app := NewServer(s, func(ctx context.Context, p string, r Request, dir string, progress func(string)) (*Result, error) {
		if !strings.Contains(p, "Original one.") {
			return nil, errors.New("missing story")
		}
		select {
		case <-gate:
			return result(), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	defer app.Close()
	body := `{"anchorId":"research-p-002","question":"What happened?","provider":"codex","model":"gpt-5.6-sol","effort":"high","parentId":""}`
	if w := request(app, "POST", "/api/jobs", body, "https://attacker.example"); w.Code != 403 {
		t.Fatal(w.Code)
	}
	r := httptest.NewRequest("GET", "http://evil.example/api/state", nil)
	w := httptest.NewRecorder()
	app.Handler().ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("host accepted")
	}
	if w := request(app, "GET", "/.research/state.json", "", testOrigin); w.Code != 404 {
		t.Fatal("arbitrary file exposed")
	}
	w = request(app, "POST", "/api/jobs", body, testOrigin)
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	var j Job
	json.Unmarshal(w.Body.Bytes(), &j)
	if w := request(app, "POST", "/api/jobs", body, testOrigin); w.Code != 400 {
		t.Fatal("parallel write accepted")
	}
	gate <- true
	awaitJob(t, s, j.ID, "completed")
	w = request(app, "POST", "/api/jobs", body, testOrigin)
	json.Unmarshal(w.Body.Bytes(), &j)
	if w := request(app, "POST", "/api/cancel", `{"id":"`+j.ID+`"}`, testOrigin); w.Code != http.StatusOK {
		t.Fatal(w.Body.String())
	}
	awaitJob(t, s, j.ID, "cancelled")
}
func TestRulesAreExplicitAndDeduplicated(t *testing.T) {
	s := fixture(t)
	j := completed(s)
	if e := s.saveLocked(true); e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(s.SkillDir, "references/follow-up-learnings.md")
	if _, e := os.Stat(path); !os.IsNotExist(e) {
		t.Fatal("rule auto-applied")
	}
	selections := []RuleSelection{{JobID: j.ID, Rule: j.Result.Learning.Rule}}
	if _, e := s.ApplyRules(selections); e != nil {
		t.Fatal(e)
	}
	if _, e := s.ApplyRules(selections); e != nil {
		t.Fatal(e)
	}
	b, _ := os.ReadFile(path)
	if strings.Count(string(b), "<!-- research-rule:") != 1 {
		t.Fatal("duplicate rule")
	}
	skill, _ := os.ReadFile(filepath.Join(s.SkillDir, "SKILL.md"))
	if !bytes.Contains(skill, []byte("Original guidance.")) {
		t.Fatal("lost original skill")
	}
}

func TestFollowUpUsesSelectedConversation(t *testing.T) {
	s := fixture(t)
	first := completed(s)
	first.ID = "first"
	first.Result.Title = "Chosen old investigation"
	for i := 0; i < 6; i++ {
		j := completed(s)
		j.ID = string(rune('b' + i))
		j.Result.Title = "Other investigation"
	}
	question := &Job{Request: Request{Question: "Explain that old result", AnchorID: s.Anchors[1].ID, ParentID: "first"}}
	prompt := s.promptLocked(question, s.Anchors[1])
	if !strings.Contains(prompt, "Chosen old investigation") || strings.Contains(prompt, `"title":"Other investigation"`) {
		t.Fatal("follow-up lost its selected parent")
	}
}
