package editor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixedClock(tm time.Time) func() time.Time { return func() time.Time { return tm } }

func TestDraftCreateGetSaveListDelete(t *testing.T) {
	dir := t.TempDir()
	ds := NewDraftStore(dir)
	ds.now = fixedClock(time.Date(2026, 6, 24, 1, 5, 30, 0, time.UTC))

	slug, form, err := ds.Create()
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if slug != "draft-20260624-010530" {
		t.Errorf("slug = %q", slug)
	}
	if form.Date != "2026/06/24" || form.Author != "Outsider" || !form.Structured {
		t.Errorf("initial form: %#v", form)
	}
	if _, err := os.Stat(filepath.Join(dir, slug+".json")); err != nil {
		t.Fatalf("draft json not written: %v", err)
	}

	form.Title = "WIP Title"
	form.ID = "2h"
	form.References = []Reference{{Text: "Go", URL: "https://go.dev"}}
	if err := ds.Save(slug, form); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := ds.Get(slug)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "WIP Title" || got.ID != "2h" || len(got.References) != 1 {
		t.Errorf("after save: %#v", got)
	}

	list, err := ds.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Slug != slug || list[0].Title != "WIP Title" || list[0].ID != "2h" {
		t.Errorf("list: %#v", list)
	}

	if err := ds.Save("draft-missing", form); !errors.Is(err, ErrNotFound) {
		t.Errorf("save missing: want ErrNotFound, got %v", err)
	}

	if err := ds.Delete(slug); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := ds.Get(slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete: %v", err)
	}
	if err := ds.Delete(slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete missing: %v", err)
	}
}

func TestDraftListEmptyWhenNoDir(t *testing.T) {
	ds := NewDraftStore(filepath.Join(t.TempDir(), "absent"))
	list, err := ds.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("want empty, got %#v", list)
	}
}

func TestDraftSlugCollisionGetsSuffix(t *testing.T) {
	ds := NewDraftStore(t.TempDir())
	ds.now = fixedClock(time.Date(2026, 6, 24, 1, 5, 30, 0, time.UTC))
	s1, _, _ := ds.Create()
	s2, _, _ := ds.Create()
	if s1 != "draft-20260624-010530" || s2 != "draft-20260624-010530-2" {
		t.Errorf("slugs = %q, %q", s1, s2)
	}
}

func TestDraftListOrdersByUpdatedDesc(t *testing.T) {
	ds := NewDraftStore(t.TempDir())
	var slugs []string
	for h := 1; h <= 3; h++ {
		ds.now = fixedClock(time.Date(2026, 6, 24, h, 0, 0, 0, time.UTC))
		s, _, _ := ds.Create()
		slugs = append(slugs, s)
	}
	list, _ := ds.List()
	if len(list) != 3 {
		t.Fatalf("want 3 drafts, got %d", len(list))
	}
	if list[0].Slug != slugs[2] || list[2].Slug != slugs[0] {
		t.Errorf("not newest-first: %s, %s, %s", list[0].Slug, list[1].Slug, list[2].Slug)
	}
}

func TestDraftPublishWritesEpisodeAndRemovesDraft(t *testing.T) {
	repo := t.TempDir()
	epDir := filepath.Join(repo, "content", "episodes")
	if err := os.MkdirAll(epDir, 0755); err != nil {
		t.Fatal(err)
	}
	episodes := NewStore(epDir)
	ds := NewDraftStore(filepath.Join(repo, "content", "drafts"))
	ds.now = fixedClock(time.Date(2026, 6, 24, 1, 5, 30, 0, time.UTC))

	slug, form, _ := ds.Create()
	form.ID = "2h"
	form.Title = "New Episode\n"
	form.EnclosureURL = "https://retrotech-episodes.outsider.dev/2h.mp3"
	form.Duration = "10:00"
	form.Intro = "intro"
	if err := ds.Save(slug, form); err != nil {
		t.Fatal(err)
	}

	id, err := ds.Publish(slug, episodes)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if id != "2h" {
		t.Errorf("published id = %q", id)
	}
	if _, err := os.Stat(filepath.Join(epDir, "2h.md")); err != nil {
		t.Errorf("episode file not written: %v", err)
	}
	if _, err := ds.Get(slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("draft should be gone after publish: %v", err)
	}
	ep, err := episodes.Get("2h")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Title != "New Episode\n" || ep.EnclosureURL != "https://retrotech-episodes.outsider.dev/2h.mp3" {
		t.Errorf("published episode: %#v", ep)
	}
}

func TestDraftPublishRejectsInvalidAndDuplicateID(t *testing.T) {
	repo := t.TempDir()
	epDir := filepath.Join(repo, "content", "episodes")
	if err := os.MkdirAll(epDir, 0755); err != nil {
		t.Fatal(err)
	}
	episodes := NewStore(epDir)
	ds := NewDraftStore(filepath.Join(repo, "content", "drafts"))

	// Empty id → invalid; the draft must survive a failed publish.
	slug, form, _ := ds.Create()
	form.ID = ""
	ds.Save(slug, form)
	if _, err := ds.Publish(slug, episodes); !errors.Is(err, ErrInvalidID) {
		t.Errorf("empty id: want ErrInvalidID, got %v", err)
	}
	if _, err := ds.Get(slug); err != nil {
		t.Errorf("draft should remain after failed publish: %v", err)
	}

	// Duplicate id → conflict.
	if err := episodes.Create(EpisodeForm{ID: "2h", Title: "existing", Structured: true}); err != nil {
		t.Fatal(err)
	}
	form.ID = "2h"
	ds.Save(slug, form)
	if _, err := ds.Publish(slug, episodes); !errors.Is(err, ErrExists) {
		t.Errorf("duplicate id: want ErrExists, got %v", err)
	}
}

// TestComposeFileHandlesEmptyFields covers publishing a barely-filled draft:
// the composed episode must still be valid, re-parseable markdown.
func TestComposeFileHandlesEmptyFields(t *testing.T) {
	form := EpisodeForm{ID: "x1", Date: "2026/06/24", Author: "Outsider", Structured: true}
	file := ComposeFile(form)
	re := reparse(t, file, "x1")
	if re.Title != "" || re.Description != "" {
		t.Errorf("empty fields not preserved: title=%q desc=%q", re.Title, re.Description)
	}
	if re.Date != "2026/06/24" || re.Author != "Outsider" {
		t.Errorf("scalar fields: %#v", re.Frontmatter)
	}
}
