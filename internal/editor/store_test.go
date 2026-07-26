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
