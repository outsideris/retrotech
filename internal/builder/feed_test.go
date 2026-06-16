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
