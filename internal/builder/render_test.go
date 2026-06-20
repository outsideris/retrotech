package builder

import (
	"strings"
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

var testSite = Site{Year: 2026}

// The nav row links between the two listing pages (home <-> episodes); a
// missing link regressed once when only one direction was rendered.
func TestHomePageNavLinksToEpisodes(t *testing.T) {
	h := BuildHomePage(nil, testSite)
	if !strings.Contains(h, `<a href="/episodes">Episodes</a>`) {
		t.Error("home nav missing the Episodes link")
	}
	if !strings.Contains(h, "<h1>RetroTech</h1>") {
		t.Error("home h1 missing")
	}
}

func TestEpisodesPageNavLinksHome(t *testing.T) {
	h := BuildEpisodesPage(nil, testSite)
	if !strings.Contains(h, `<a href="/">RetroTech</a>`) {
		t.Error("episodes nav missing the home (RetroTech) link")
	}
	if !strings.Contains(h, "<h1>Episodes</h1>") {
		t.Error("episodes h1 missing")
	}
}

func TestEpisodePageRendersTitleAndBadges(t *testing.T) {
	ep := parser.Episode{
		Frontmatter: parser.Frontmatter{Title: "2g. VCS: SourceForge", Date: "2026/03/07", Author: "Outsider"},
		ID:          "2g",
		Body:        "intro paragraph\n\n<!--badges-->\n",
	}
	h := BuildEpisodePage(ep, testSite)
	if !strings.Contains(h, "<h1>2g. VCS: SourceForge</h1>") {
		t.Error("episode h1 missing")
	}
	if !strings.Contains(h, `<div class="badges">`) {
		t.Error("badges not injected at the marker")
	}
	if strings.Contains(h, "<!--badges-->") {
		t.Error("badges marker was left unreplaced")
	}
	// The footer (host profile) lives inside the article.
	if !strings.Contains(h, "<footer>") {
		t.Error("footer missing")
	}
}

// Every page needs exactly one main landmark and a skip link pointing at it
// (the old Nextra build's role="main" fix that the migration must keep).
func TestPageHasMainLandmarkAndSkipLink(t *testing.T) {
	h := BuildHomePage(nil, testSite)
	if !strings.Contains(h, `role="main"`) || !strings.Contains(h, `id="content"`) {
		t.Error("missing main landmark (role=main / id=content)")
	}
	if !strings.Contains(h, `<a class="skip-link" href="#content">`) {
		t.Error("missing skip link to #content")
	}
	if strings.Count(h, `role="main"`) != 1 {
		t.Errorf("want exactly one main landmark, got %d", strings.Count(h, `role="main"`))
	}
}

// The references list is authored as plain markdown under "#### 레퍼런스:"; the
// builder wraps it in <div class="refs"> so no raw HTML lives in the content.
func TestReferencesAutoWrapped(t *testing.T) {
	ep := parser.Episode{
		Frontmatter: parser.Frontmatter{Title: "x", Date: "2026/03/07", Author: "Outsider"},
		ID:          "x",
		Body:        "intro\n\n<!--badges-->\n\n## 레퍼런스:\n\n* [a](https://example.com)\n* [b](https://example.org)\n",
	}
	h := BuildEpisodePage(ep, testSite)
	if !strings.Contains(h, `<div class="refs"><ul>`) {
		t.Errorf("references list not wrapped in .refs:\n%s", h)
	}
	if !strings.Contains(h, "</ul></div>") {
		t.Error("refs wrapper not closed")
	}
}
