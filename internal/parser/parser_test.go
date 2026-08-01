package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitFrontmatterAndBody(t *testing.T) {
	in := []byte("---\ntitle: Hello\n---\n\n# Body\n\ntext\n")
	fm, body, err := SplitFrontmatterAndBody(in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(fm) != "title: Hello" {
		t.Errorf("frontmatter = %q", fm)
	}
	if string(body) != "# Body\n\ntext" {
		t.Errorf("body = %q", body)
	}
}

func TestSplitFrontmatterMissing(t *testing.T) {
	if _, _, err := SplitFrontmatterAndBody([]byte("no frontmatter here")); err == nil {
		t.Fatal("expected error for missing frontmatter")
	}
}

// The feed embeds the title verbatim, so the trailing newline a folded ">"
// scalar produces (and its absence for a quoted scalar) must survive parsing
// exactly — this is what keeps <title> byte-identical to the current feed.
func TestParseFrontmatterTitleScalarForms(t *testing.T) {
	folded, err := ParseFrontmatter([]byte("title: >\n    2g. VCS: SourceForge\n"))
	if err != nil {
		t.Fatalf("folded: %v", err)
	}
	if folded.Title != "2g. VCS: SourceForge\n" {
		t.Errorf("folded title = %q, want trailing newline", folded.Title)
	}

	quoted, err := ParseFrontmatter([]byte(`title: "0. Pilot"`))
	if err != nil {
		t.Fatalf("quoted: %v", err)
	}
	if quoted.Title != "0. Pilot" {
		t.Errorf("quoted title = %q, want no trailing newline", quoted.Title)
	}
}

func TestParseFrontmatterFields(t *testing.T) {
	src := []byte(`title: "2g"
date: 2026/03/07
description: |
    line one
    line two
description2: |
    extra
author: Outsider
enclosure:
  url: https://retrotech-episodes.outsider.dev/2g.mp3
  size: 66997696
duration: "55:50"
`)
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Description != "line one\nline two\n" {
		t.Errorf("description = %q", fm.Description)
	}
	if fm.Description2 != "extra\n" {
		t.Errorf("description2 = %q", fm.Description2)
	}
	if fm.Enclosure.URL != "https://retrotech-episodes.outsider.dev/2g.mp3" || fm.Enclosure.Size != 66997696 {
		t.Errorf("enclosure = %+v", fm.Enclosure)
	}
	if fm.Duration != "55:50" {
		t.Errorf("duration = %q", fm.Duration)
	}
	if got := fm.ParsedDate().Format("2006-01-02"); got != "2026-03-07" {
		t.Errorf("ParsedDate = %s", got)
	}
}

func TestParseFrontmatterChapters(t *testing.T) {
	src := []byte(`title: "2g"
date: 2026/03/07
chapters:
  - start: "00:00"
    title: 인트로
  - start: "03:15"
    title: SourceForge의 시작
  - start: "1:02:33"
    title: 마무리
`)
	fm, err := ParseFrontmatter(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fm.Chapters) != 3 {
		t.Fatalf("chapters = %d, want 3", len(fm.Chapters))
	}
	if fm.Chapters[1].Start != "03:15" || fm.Chapters[1].Title != "SourceForge의 시작" {
		t.Errorf("chapters[1] = %+v", fm.Chapters[1])
	}
}

func TestChapterStartSeconds(t *testing.T) {
	valid := []struct {
		start string
		want  int
	}{
		{"00:00", 0},
		{"3:15", 195},
		{"55:50", 3350},
		{"75:12", 4512}, // minutes beyond 59, like the duration field
		{"1:02:33", 3753},
		{" 02:00 ", 120},
	}
	for _, tc := range valid {
		got, err := (Chapter{Start: tc.start}).StartSeconds()
		if err != nil {
			t.Errorf("StartSeconds(%q) error: %v", tc.start, err)
			continue
		}
		if got != tc.want {
			t.Errorf("StartSeconds(%q) = %d, want %d", tc.start, got, tc.want)
		}
	}

	invalid := []string{"", "12", "abc", "1:xx", "1:60", "-1:00", "1:-2", "1:2:3:4"}
	for _, start := range invalid {
		if _, err := (Chapter{Start: start}).StartSeconds(); err == nil {
			t.Errorf("StartSeconds(%q) succeeded, want error", start)
		}
	}
}

// A malformed chapter must fail the load (and therefore the build) with the
// file named, instead of surfacing later as a broken chapters JSON.
func TestLoadEpisodeRejectsBadChapters(t *testing.T) {
	cases := map[string]string{
		"bad start":   "---\ntitle: x\ndate: 2026/01/01\nchapters:\n  - start: \"oops\"\n    title: t\n---\nbody",
		"empty title": "---\ntitle: x\ndate: 2026/01/01\nchapters:\n  - start: \"00:00\"\n    title: \"\"\n---\nbody",
	}
	for name, src := range cases {
		path := filepath.Join(t.TempDir(), "bad.md")
		if err := os.WriteFile(path, []byte(src), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadEpisode(path); err == nil {
			t.Errorf("%s: LoadEpisode succeeded, want error", name)
		}
	}
}

func TestLoadEpisodesOrderAndSkip(t *testing.T) {
	dir := t.TempDir()
	write := func(name, date string) {
		body := "---\ntitle: \"" + name + "\"\ndate: " + date + "\n---\n\nbody\n"
		if err := os.WriteFile(filepath.Join(dir, name+".md"), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("2a", "2025/05/11")
	write("2b", "2025/06/08")
	write("0", "2023/07/24")
	// index listing page must be skipped.
	if err := os.WriteFile(filepath.Join(dir, "index.md"), []byte("---\ntitle: Episodes\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	eps, err := LoadEpisodes(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := make([]string, len(eps))
	for i, e := range eps {
		got[i] = e.ID
	}
	want := []string{"2b", "2a", "0"} // newest first
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", got, want)
	}
}
