package editor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

func sampleForm() EpisodeForm {
	return EpisodeForm{
		ID:            "9z",
		Title:         "Test Episode\n",
		Date:          "2026/06/21",
		Description:   "테스트 설명\n",
		EnclosureURL:  "https://retrotech-episodes.outsider.dev/9z.mp3",
		EnclosureSize: 12345678,
		Duration:      "12:34",
		Badges:        parser.Badges{Apple: "https://podcasts.apple.com/x"},
		Structured:    true,
		Intro:         "intro paragraph",
		References: []Reference{
			{Text: "Go", URL: "https://go.dev"},
			{Text: "section label"},
			{Text: "nested", URL: "https://example.com", Indent: 1},
		},
		Extra: "## 배경음악\nMusic from #Uppbeat",
	}
}

func TestStoreCreateGetUpdateDelete(t *testing.T) {
	s := NewStore(t.TempDir())
	form := sampleForm()

	if err := s.Create(form); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.Create(form); !errors.Is(err, ErrExists) {
		t.Fatalf("duplicate create: want ErrExists, got %v", err)
	}

	got, err := s.Get("9z")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "Test Episode\n" {
		t.Errorf("title: got %q", got.Title)
	}
	if got.EnclosureSize != 12345678 || got.Duration != "12:34" {
		t.Errorf("enclosure/duration: got %d / %q", got.EnclosureSize, got.Duration)
	}
	if got.Badges.Apple != "https://podcasts.apple.com/x" {
		t.Errorf("badge: got %q", got.Badges.Apple)
	}
	if !got.Structured || got.Intro != "intro paragraph" {
		t.Errorf("structured/intro: %v / %q", got.Structured, got.Intro)
	}
	if len(got.References) != 3 || got.References[2].Indent != 1 || got.References[2].URL != "https://example.com" {
		t.Errorf("references not preserved: %#v", got.References)
	}
	if !strings.Contains(got.Extra, "배경음악") {
		t.Errorf("extra: got %q", got.Extra)
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != "9z" || list[0].Duration != "12:34" {
		t.Errorf("list: %#v", list)
	}

	form.Duration = "20:00"
	if err := s.Update("9z", form); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got, _ := s.Get("9z"); got.Duration != "20:00" {
		t.Errorf("after update, duration = %q", got.Duration)
	}
	if err := s.Update("missing", form); !errors.Is(err, ErrNotFound) {
		t.Fatalf("update missing: want ErrNotFound, got %v", err)
	}

	if err := s.Delete("9z"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.Get("9z"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get after delete: want ErrNotFound, got %v", err)
	}
	if err := s.Delete("9z"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("delete missing: want ErrNotFound, got %v", err)
	}
}

// TestUpdateIgnoresBodyID ensures the URL/guid can't be changed via the request
// body: Update keys off the path id, not the form's ID field.
func TestUpdateIgnoresBodyID(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	form := sampleForm()
	if err := s.Create(form); err != nil {
		t.Fatalf("create: %v", err)
	}

	form.ID = "hacked"
	form.Title = "Renamed\n"
	if err := s.Update("9z", form); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hacked.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("update wrote to body id, not path id")
	}
	if got, _ := s.Get("9z"); got.Title != "Renamed\n" {
		t.Errorf("update did not apply to path id: %q", got.Title)
	}
}

// TestStoreDerivesDescriptionFromIntro: for structured forms the description is
// never the caller's — Create derives it from the intro with links stripped.
func TestStoreDerivesDescriptionFromIntro(t *testing.T) {
	s := NewStore(t.TempDir())
	form := sampleForm()
	form.Description = "무시되어야 하는 값"
	form.Intro = "[Go](https://go.dev)로 만든 도구 이야기."

	if err := s.Create(form); err != nil {
		t.Fatalf("create: %v", err)
	}
	got, err := s.Get("9z")
	if err != nil {
		t.Fatal(err)
	}
	if want := "Go로 만든 도구 이야기.\n"; got.Description != want {
		t.Errorf("description: got %q, want %q", got.Description, want)
	}
	if got.Intro != "[Go](https://go.dev)로 만든 도구 이야기." {
		t.Errorf("intro should keep its links: %q", got.Intro)
	}
}

// TestUpdatePreservesLegacyDescription: legacy episodes wrap their description
// differently from their intro; an update that doesn't touch the intro must
// keep the stored description byte-exact (feed stability), while an intro edit
// re-derives it.
func TestUpdatePreservesLegacyDescription(t *testing.T) {
	dir := t.TempDir()
	legacy := `---
title: >
    9z. Legacy
date: 2025/01/01
description: |
    한 줄로 된 설명입니다.
enclosure:
  url: https://retrotech-episodes.outsider.dev/9z.mp3
  size: 1
duration: "1:00"
---

한 줄로
된 설명입니다.

<!--badges-->
`
	if err := os.WriteFile(filepath.Join(dir, "9z.md"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)

	form, err := s.Get("9z")
	if err != nil {
		t.Fatal(err)
	}
	if !form.Structured {
		t.Fatalf("legacy episode should be structured: %#v", form)
	}

	// Unrelated edit: the differently-wrapped description survives byte-exact.
	form.Duration = "2:00"
	if err := s.Update("9z", form); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.Get("9z")
	if want := "한 줄로 된 설명입니다.\n"; got.Description != want {
		t.Errorf("untouched intro: description = %q, want %q", got.Description, want)
	}

	// Intro edit: the description follows.
	got.Intro = "완전히 새로운 도입부입니다."
	if err := s.Update("9z", got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = s.Get("9z")
	if want := "완전히 새로운 도입부입니다.\n"; got.Description != want {
		t.Errorf("edited intro: description = %q, want %q", got.Description, want)
	}
}

func TestStoreRejectsUnsafeIDs(t *testing.T) {
	s := NewStore(t.TempDir())
	unsafe := []string{
		"", "../escape", "a/b", ".hidden", "with space",
		"sub/../../etc", "..", "trailing/", strings.Repeat("x", 65),
	}
	for _, id := range unsafe {
		if _, err := s.Get(id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Get(%q): want ErrInvalidID, got %v", id, err)
		}
		if err := s.Create(EpisodeForm{ID: id}); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Create(%q): want ErrInvalidID, got %v", id, err)
		}
		if err := s.Delete(id); !errors.Is(err, ErrInvalidID) {
			t.Errorf("Delete(%q): want ErrInvalidID, got %v", id, err)
		}
	}
}

func TestValidID(t *testing.T) {
	for _, id := range []string{"0", "2g", "1a", "1n", "250127-breaks", "abc_DEF-123"} {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false, want true", id)
		}
	}
	for _, id := range []string{"", "-lead", "_lead", "a/b", "../x", "a.b", ".x", "a b"} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true, want false", id)
		}
	}
}

// The episode file carries the HTML the feed ships (feedDescription) so the
// markdown shows exactly what subscribers receive. Create always writes it;
// Update keeps the stored value while its sources are untouched, so an episode
// written before the field existed does not gain one on an unrelated edit.
func TestStoreWritesFeedDescription(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(dir)
	form := sampleForm()
	form.Intro = "첫 문장.\n둘째 문장."
	form.Description2 = "레퍼런스는 홈페이지 참고:\nhttps://retrotech.outsider.dev/episodes/9z\n"

	if err := s.Create(form); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := s.Get("9z")
	if err != nil {
		t.Fatal(err)
	}
	want := "<p>첫 문장.<br/>둘째 문장.</p><p>레퍼런스는 홈페이지 참고:<br/>" +
		`<a href="https://retrotech.outsider.dev/episodes/9z">https://retrotech.outsider.dev/episodes/9z</a></p>` + "\n"
	if got.FeedDescription != want {
		t.Errorf("create: feedDescription = %q, want %q", got.FeedDescription, want)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "9z.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "feedDescription: |\n    <p>첫 문장.<br/>둘째 문장.</p>") {
		t.Errorf("feedDescription is not in the file as a block scalar:\n%s", raw)
	}

	// An unrelated edit leaves the stored HTML alone.
	got.Duration = "99:99"
	if err := s.Update("9z", got); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ := s.Get("9z")
	if after.FeedDescription != want {
		t.Errorf("unrelated edit changed feedDescription: %q", after.FeedDescription)
	}

	// Editing the intro re-derives the description, and the HTML follows.
	after.Intro = "완전히 새로운 도입부입니다."
	if err := s.Update("9z", after); err != nil {
		t.Fatalf("update: %v", err)
	}
	after, _ = s.Get("9z")
	if !strings.HasPrefix(after.FeedDescription, "<p>완전히 새로운 도입부입니다.</p><p>레퍼런스는") {
		t.Errorf("intro edit did not regenerate feedDescription: %q", after.FeedDescription)
	}
}

// An episode file written before feedDescription existed must not gain one from
// an edit that leaves both description fields alone — otherwise an unrelated
// save would churn the feed for every legacy episode.
func TestUpdateLeavesLegacyFileWithoutFeedDescription(t *testing.T) {
	dir := t.TempDir()
	legacy := `---
title: >
    9z. Legacy
date: 2025/01/01
description: |
    한 줄로 된 설명입니다.
enclosure:
  url: https://retrotech-episodes.outsider.dev/9z.mp3
  size: 1
duration: "1:00"
---

한 줄로
된 설명입니다.

<!--badges-->
`
	if err := os.WriteFile(filepath.Join(dir, "9z.md"), []byte(legacy), 0644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(dir)

	form, err := s.Get("9z")
	if err != nil {
		t.Fatal(err)
	}
	form.Duration = "2:00"
	if err := s.Update("9z", form); err != nil {
		t.Fatalf("update: %v", err)
	}

	got, _ := s.Get("9z")
	if got.FeedDescription != "" {
		t.Errorf("unrelated edit added feedDescription to a legacy file: %q", got.FeedDescription)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "9z.md"))
	if strings.Contains(string(raw), "feedDescription") {
		t.Errorf("legacy file gained a feedDescription key:\n%s", raw)
	}
}
