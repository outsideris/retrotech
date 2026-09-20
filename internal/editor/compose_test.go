package editor

import (
	"bytes"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/outsideris/retrotech/internal/builder"
	"github.com/outsideris/retrotech/internal/parser"
)

// contentEpisodesDir is the real episode corpus, relative to this package.
func contentEpisodesDir() string {
	return filepath.Join("..", "..", "content", "episodes")
}

func loadCorpus(t *testing.T) []parser.Episode {
	t.Helper()
	eps, err := parser.LoadEpisodes(contentEpisodesDir())
	if err != nil {
		t.Fatalf("load episodes: %v", err)
	}
	if len(eps) == 0 {
		t.Fatal("no episodes found in corpus")
	}
	return eps
}

// reparse reconstructs an Episode from composed file bytes the same way
// parser.LoadEpisode does from disk.
func reparse(t *testing.T, data []byte, id string) parser.Episode {
	t.Helper()
	fmBytes, bodyBytes, err := parser.SplitFrontmatterAndBody(data)
	if err != nil {
		t.Fatalf("split composed file for %s: %v", id, err)
	}
	fm, err := parser.ParseFrontmatter(fmBytes)
	if err != nil {
		t.Fatalf("parse composed frontmatter for %s: %v", id, err)
	}
	return parser.Episode{Frontmatter: fm, ID: id, Body: string(bodyBytes)}
}

// TestRoundTripPreservesEveryEpisode is the core safety net: turning each real
// episode into a form and composing it back must preserve the frontmatter
// values exactly (the feed contract) and the body bytes exactly. Structured
// episodes additionally rebuild their body from the structured fields, not the
// raw fallback.
func TestRoundTripPreservesEveryEpisode(t *testing.T) {
	for _, ep := range loadCorpus(t) {
		ep := ep
		t.Run(ep.ID, func(t *testing.T) {
			form := EpisodeToForm(ep)

			if form.Structured {
				if got := composeBody(form); got != ep.Body {
					t.Errorf("structured body not byte-identical\n--- got ---\n%q\n--- want ---\n%q", got, ep.Body)
				}
			}

			file := ComposeFile(form)
			re := reparse(t, file, ep.ID)

			if !reflect.DeepEqual(re.Frontmatter, ep.Frontmatter) {
				t.Errorf("frontmatter values changed\n got: %#v\nwant: %#v", re.Frontmatter, ep.Frontmatter)
			}
			if re.Body != ep.Body {
				t.Errorf("body changed after file round-trip\n--- got ---\n%q\n--- want ---\n%q", re.Body, ep.Body)
			}
		})
	}
}

// TestComposedEpisodesProduceIdenticalFeed guards the subscriber contract
// directly: the RSS feed built from episodes that have been round-tripped
// through the editor must be byte-for-byte identical to the feed built from the
// originals (the feed golden test in internal/builder enforces the absolute
// bytes; this enforces that the editor never perturbs them).
func TestComposedEpisodesProduceIdenticalFeed(t *testing.T) {
	original := loadCorpus(t)

	recomposed := make([]parser.Episode, len(original))
	for i, ep := range original {
		recomposed[i] = reparse(t, ComposeFile(EpisodeToForm(ep)), ep.ID)
	}

	cfg := builder.FeedConfig{SiteURL: "https://retrotech.outsider.dev"}
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	want := builder.BuildFeed(original, cfg, at)
	got := builder.BuildFeed(recomposed, cfg, at)
	if !bytes.Equal(want, got) {
		t.Errorf("feed changed after editor round-trip:\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

// TestBlockScalarPreservesTrailingNewlines checks the chomping logic in
// isolation: each value, emitted as a literal block scalar and re-parsed as
// YAML, must come back exactly — including the trailing-newline count that the
// feed reproduces verbatim.
func TestBlockScalarPreservesTrailingNewlines(t *testing.T) {
	values := []string{
		"single line no newline",   // strip  (|-)
		"single line\n",            // clip   (|)
		"para one\n\npara two\n",   // clip, internal blank line
		"a\nb\nc",                  // multiline, no trailing newline
		"trailing blanks\n\n",      // keep   (|+)
		"VCS: SCCS, colon space\n", // ": " would break an inline scalar
	}
	for _, v := range values {
		v := v
		t.Run(v, func(t *testing.T) {
			// Embed the scalar in a minimal but complete frontmatter document.
			doc := "title: t\n" +
				blockScalar("description", v) +
				"date: 2024/1/1\nauthor: a\nenclosure:\n  url: u\n  size: 1\nduration: \"1:00\"\n"
			fm, err := parser.ParseFrontmatter([]byte(doc))
			if err != nil {
				t.Fatalf("parse: %v\ndoc:\n%s", err, doc)
			}
			if fm.Description != v {
				t.Errorf("description round-trip: got %q want %q", fm.Description, v)
			}
		})
	}
}

// TestParseLinkItemIsInvertible verifies that re-composing a parsed bullet
// reproduces the original text — the property body round-tripping relies on,
// including URLs that themselves contain parentheses and non-link plain text.
func TestParseLinkItemIsInvertible(t *testing.T) {
	items := []string{
		"[XSFM](http://xsfm.co.kr/)",
		"[John McCarthy - Wikipedia](https://en.wikipedia.org/wiki/John_McCarthy_(computer_scientist))",
		"[Backus–Naur form - Wikipedia](https://en.wikipedia.org/wiki/Backus%E2%80%93Naur_form)",
		"Svelte",
		"[unclosed link",
	}
	for _, item := range items {
		text, url := parseLinkItem(item)
		got := text
		if url != "" {
			got = "[" + text + "](" + url + ")"
		}
		if got != item {
			t.Errorf("parseLinkItem not invertible: %q -> %q", item, got)
		}
	}
}

// TestComposeBodyOmitsEmptySections checks the section-omission rules that keep
// minimal episodes (a "Breaks" filler with no reference list) from gaining a
// stray "## 레퍼런스:" heading on save.
func TestComposeBodyOmitsEmptySections(t *testing.T) {
	got := composeBody(EpisodeForm{Structured: true, Intro: "just a note"})
	want := "just a note\n\n" + badgesMarker
	if got != want {
		t.Errorf("empty-section body: got %q want %q", got, want)
	}

	withRefs := composeBody(EpisodeForm{
		Structured: true,
		Intro:      "intro",
		References: []Reference{{Text: "Go", URL: "https://go.dev"}},
		Extra:      "## 배경음악\nMusic",
	})
	wantRefs := "intro\n\n" + badgesMarker + "\n\n" + refsHeading + "\n\n* [Go](https://go.dev)\n\n## 배경음악\nMusic"
	if withRefs != wantRefs {
		t.Errorf("full body: got %q want %q", withRefs, wantRefs)
	}
}

// TestChaptersSurviveFrontmatterRoundTrip guards the chapter list explicitly,
// rather than relying on the corpus happening to contain a chaptered episode:
// a form's chapters must re-parse identically, including the hazards a naive
// emitter gets wrong — a bare "MM:SS" start is sexagesimal YAML (02:41 -> 161),
// and a title may begin with a non-ASCII character or contain ": ", "#" or a
// quote. Losing chapters here silently drops <podcast:chapters> and the
// timestamp lines from every subscriber's feed.
func TestChaptersSurviveFrontmatterRoundTrip(t *testing.T) {
	chapters := []parser.Chapter{
		{Start: "00:00", Title: "Intro"},
		{Start: "02:41", Title: "1990년대의 Linux 생태계"},
		{Start: "10:00", Title: "또 시작된 패치 지연 이슈"},         // leading non-ASCII
		{Start: "24:05", Title: "VCS: SCCS, colon space"}, // ": " breaks a plain scalar
		{Start: "68:07", Title: `a "quoted" # hash`},      // quote + comment marker
		{Start: "1:15:44", Title: "HH:MM:SS form"},
		{Start: "96:05", Title: "true"}, // reserved word must stay a string
	}

	form := EpisodeForm{
		Title: "t", Date: "2026/09/20", Description: "d\n",
		EnclosureURL: "https://example.com/x.mp3", EnclosureSize: 1, Duration: "99:16",
		Chapters: chapters,
	}
	fm, err := parser.ParseFrontmatter([]byte(composeFrontmatter(form)))
	if err != nil {
		t.Fatalf("parse composed frontmatter: %v\n%s", err, composeFrontmatter(form))
	}
	if !reflect.DeepEqual(fm.Chapters, chapters) {
		t.Errorf("chapters changed\n got: %#v\nwant: %#v", fm.Chapters, chapters)
	}

	// Starts must survive as strings, not as the integers YAML would infer.
	for i, want := range []int{0, 161, 600, 1445, 4087, 4544, 5765} {
		got, err := fm.Chapters[i].StartSeconds()
		if err != nil {
			t.Errorf("chapter %d StartSeconds: %v", i, err)
		} else if got != want {
			t.Errorf("chapter %d start %q -> %d seconds, want %d", i, fm.Chapters[i].Start, got, want)
		}
	}
}

// TestComposeOmitsChaptersWhenEmpty keeps episodes without chapters from
// gaining an empty "chapters:" key on save.
func TestComposeOmitsChaptersWhenEmpty(t *testing.T) {
	form := EpisodeForm{
		Title: "t", Date: "2026/09/20", Description: "d\n",
		EnclosureURL: "https://example.com/x.mp3", EnclosureSize: 1, Duration: "10:00",
	}
	if got := composeFrontmatter(form); strings.Contains(got, "chapters") {
		t.Errorf("chapterless episode gained a chapters key:\n%s", got)
	}
}
