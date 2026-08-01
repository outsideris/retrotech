package builder

import (
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

func TestBuildChaptersJSON(t *testing.T) {
	ep := parser.Episode{
		ID: "2g",
		Frontmatter: parser.Frontmatter{
			Chapters: []parser.Chapter{
				{Start: "00:00", Title: "인트로"},
				{Start: "03:15", Title: " SourceForge의 시작 "},
				{Start: "1:02:33", Title: "마무리"},
			},
		},
	}

	got, err := BuildChaptersJSON(ep)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `{
  "version": "1.2.0",
  "chapters": [
    {
      "startTime": 0,
      "title": "인트로"
    },
    {
      "startTime": 195,
      "title": "SourceForge의 시작"
    },
    {
      "startTime": 3753,
      "title": "마무리"
    }
  ]
}
`
	if string(got) != want {
		t.Errorf("chapters JSON mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestBuildChaptersJSONNoChapters(t *testing.T) {
	got, err := BuildChaptersJSON(parser.Episode{ID: "0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for an episode without chapters, got %q", got)
	}
}

func TestBuildChaptersJSONBadStart(t *testing.T) {
	ep := parser.Episode{
		ID: "x",
		Frontmatter: parser.Frontmatter{
			Chapters: []parser.Chapter{{Start: "oops", Title: "t"}},
		},
	}
	if _, err := BuildChaptersJSON(ep); err == nil {
		t.Fatal("expected error for invalid chapter start")
	}
}

func TestChaptersRelPath(t *testing.T) {
	if got := ChaptersRelPath("2g"); got != "episodes/2g.chapters.json" {
		t.Errorf("ChaptersRelPath = %q", got)
	}
}
