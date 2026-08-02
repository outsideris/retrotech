package builder

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/outsideris/retrotech/internal/parser"
)

// episodeSourceDir is where episode markdown lives.
const episodeSourceDir = "../../content/episodes"

// lastBuildDate is the one feed element that legitimately changes every build,
// so it is normalised out before comparing against the golden.
var lastBuildDateRE = regexp.MustCompile(`<lastBuildDate>[^<]*</lastBuildDate>`)

func normalizeFeed(s string) string {
	return lastBuildDateRE.ReplaceAllString(s, "<lastBuildDate>X</lastBuildDate>")
}

// loadFrontmatterEpisodes reads every episode file's frontmatter (the feed
// needs no rendered body), tolerating both .md and the current .mdx source so
// the golden check runs before the content is converted.
func loadFrontmatterEpisodes(t *testing.T, dir string) []parser.Episode {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var eps []parser.Episode
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := filepath.Ext(name)
		if ext != ".md" && ext != ".mdx" {
			continue
		}
		if strings.HasPrefix(name, "index.") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		fmBytes, _, err := parser.SplitFrontmatterAndBody(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fm, err := parser.ParseFrontmatter(fmBytes)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		eps = append(eps, parser.Episode{Frontmatter: fm, ID: strings.TrimSuffix(name, ext)})
	}
	return eps
}

// TestBuildFeedMatchesGolden asserts the Go feed is byte-identical (modulo the
// volatile lastBuildDate) to the feed.xml the old scripts/gen-rss.js produced —
// the subscriber contract (guid/enclosure/pubDate) and every other field.
func TestBuildFeedMatchesGolden(t *testing.T) {
	golden, err := os.ReadFile("testdata/feed.golden.xml")
	if err != nil {
		t.Fatal(err)
	}

	eps := loadFrontmatterEpisodes(t, episodeSourceDir)
	if len(eps) == 0 {
		t.Fatal("no episodes found")
	}

	got := BuildFeed(eps, FeedConfig{SiteURL: "https://retrotech.outsider.dev"}, time.Now())

	g := normalizeFeed(string(got))
	w := normalizeFeed(string(golden))
	if g == w {
		return
	}

	// Report the first divergence with surrounding context.
	n := len(g)
	if len(w) < n {
		n = len(w)
	}
	i := 0
	for i < n && g[i] == w[i] {
		i++
	}
	t.Fatalf("feed mismatch at byte %d (got %d bytes, want %d bytes)\n--- got  ---\n%q\n--- want ---\n%q",
		i, len(g), len(w), context(g, i), context(w, i))
}

// Chapters must surface in the feed two ways — a <podcast:chapters> link (with
// the namespace declared) for Podcasting 2.0 apps, and plain "MM:SS title"
// lines appended to the description for apps that auto-link timestamps — while
// episodes without chapters stay untouched.
func TestBuildFeedWithChapters(t *testing.T) {
	withChapters := parser.Episode{
		ID: "2h",
		Frontmatter: parser.Frontmatter{
			Title:       "2h. Test",
			Date:        "2026/07/01",
			Description: "요약.\n",
			Author:      "Outsider",
			Chapters: []parser.Chapter{
				{Start: "00:00", Title: "인트로"},
				{Start: "03:15", Title: "본론"},
			},
		},
	}
	plain := parser.Episode{
		ID: "2g",
		Frontmatter: parser.Frontmatter{
			Title:       "2g. Plain",
			Date:        "2026/03/07",
			Description: "요약",
			Author:      "Outsider",
		},
	}

	feed := string(BuildFeed([]parser.Episode{withChapters, plain}, FeedConfig{SiteURL: "https://retrotech.outsider.dev"}, time.Now()))

	if !strings.Contains(feed, `xmlns:podcast="https://podcastindex.org/namespace/1.0"`) {
		t.Error("feed missing podcast namespace declaration")
	}
	if !strings.Contains(feed, `<podcast:chapters url="https://retrotech.outsider.dev/episodes/2h.chapters.json" type="application/json+chapters"/>`) {
		t.Error("feed missing <podcast:chapters> for the chaptered episode")
	}
	if !strings.Contains(feed, "요약.\n\n00:00 인트로\n03:15 본론") {
		t.Error("feed description missing appended chapter timestamp lines")
	}
	if strings.Count(feed, "podcast:chapters") != 1 {
		t.Error("<podcast:chapters> leaked into the chapterless episode")
	}
}

// Without any chaptered episode the feed must not change at all — no podcast
// namespace, no chapter lines. (The golden test pins the full byte form; this
// pins the reason it still passes.)
func TestBuildFeedWithoutChaptersUnchanged(t *testing.T) {
	ep := parser.Episode{
		ID: "2g",
		Frontmatter: parser.Frontmatter{
			Title:       "2g. Plain",
			Date:        "2026/03/07",
			Description: "요약",
			Author:      "Outsider",
		},
	}
	feed := string(BuildFeed([]parser.Episode{ep}, FeedConfig{SiteURL: "https://retrotech.outsider.dev"}, time.Now()))
	if strings.Contains(feed, "xmlns:podcast") {
		t.Error("chapterless feed must not declare the podcast namespace")
	}
	if strings.Contains(feed, "podcast:chapters") {
		t.Error("chapterless feed must not emit <podcast:chapters>")
	}
}

func context(s string, i int) string {
	a, b := i-100, i+100
	if a < 0 {
		a = 0
	}
	if b > len(s) {
		b = len(s)
	}
	return s[a:b]
}
