package builder

import (
	"regexp"
	"strings"
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

// These tests pin the accessibility and performance markup that the Lighthouse
// audits (run manually + in the Lighthouse-CI job) reward, so a regression is
// caught fast and deterministically by `go test` — without needing a browser.
// They assert the *invariants that produce* the scores (one main landmark, no
// skipped heading levels, alt text, lazy iframes, reserved image heights, the
// LCP preload), not the scores themselves.

// invariantPages renders one of every page type with representative content so
// the invariants below run against the real builder output.
func invariantPages(t *testing.T) map[string]string {
	t.Helper()
	eps := []parser.Episode{
		{
			Frontmatter: parser.Frontmatter{Title: "2g. VCS: SourceForge", Date: "2026/03/07", Author: "Outsider", Description: "소스포지 이야기"},
			ID:          "2g",
			Body:        "intro paragraph\n\n<!--badges-->\n\n## 레퍼런스:\n\n* [a](https://example.com)\n* [b](https://example.org)\n",
		},
		{
			Frontmatter: parser.Frontmatter{Title: "1a. 시작", Date: "2026/01/01", Author: "Outsider", Description: "첫 회"},
			ID:          "1a",
			Body:        "intro\n\n<!--badges-->\n",
		},
	}
	return map[string]string{
		"home":     BuildHomePage(eps, testSite),
		"episodes": BuildEpisodesPage(eps, testSite),
		"episode":  BuildEpisodePage(eps[0], testSite),
		"404":      Build404Page(testSite),
	}
}

var (
	headingTagRE = regexp.MustCompile(`(?i)<h([1-6])[\s>]`)
	imgTagRE     = regexp.MustCompile(`(?i)<img\b[^>]*>`)
	iframeTagRE  = regexp.MustCompile(`(?i)<iframe\b[^>]*>`)
)

func headingLevels(htmlStr string) []int {
	ms := headingTagRE.FindAllStringSubmatch(htmlStr, -1)
	levels := make([]int, len(ms))
	for i, m := range ms {
		levels[i] = int(m[1][0] - '0')
	}
	return levels
}

// TestHeadingOrderNoSkips guards axe's heading-order rule: every page starts at
// a single h1 and never jumps more than one level deeper (h1→h3 is the failure
// that the /episodes list and /404 footer used to trigger; both now bridge with
// a visually-hidden h2).
func TestHeadingOrderNoSkips(t *testing.T) {
	for name, h := range invariantPages(t) {
		levels := headingLevels(h)
		if len(levels) == 0 {
			t.Errorf("%s: no headings found", name)
			continue
		}
		if levels[0] != 1 {
			t.Errorf("%s: first heading is h%d, want h1", name, levels[0])
		}
		h1Count := 0
		for _, l := range levels {
			if l == 1 {
				h1Count++
			}
		}
		if h1Count != 1 {
			t.Errorf("%s: want exactly one h1, got %d (levels %v)", name, h1Count, levels)
		}
		for i := 1; i < len(levels); i++ {
			if levels[i] > levels[i-1]+1 {
				t.Errorf("%s: heading level skip h%d → h%d (levels %v)", name, levels[i-1], levels[i], levels)
				break
			}
		}
	}
}

// TestEveryImageHasAlt guards axe's image-alt rule: every <img> declares an alt
// attribute (decorative images would use alt="", but this site has none, so a
// missing alt is always a bug).
func TestEveryImageHasAlt(t *testing.T) {
	for name, h := range invariantPages(t) {
		imgs := imgTagRE.FindAllString(h, -1)
		if len(imgs) == 0 {
			continue
		}
		for _, tag := range imgs {
			if !strings.Contains(tag, "alt=") {
				t.Errorf("%s: <img> without alt attribute: %s", name, tag)
			}
		}
	}
}

// TestSingleMainLandmark guards landmark-one-main + the skip link on every page
// type (the home-only check lives in render_test.go; this extends it).
func TestSingleMainLandmark(t *testing.T) {
	for name, h := range invariantPages(t) {
		if n := strings.Count(h, `role="main"`); n != 1 {
			t.Errorf("%s: want exactly one role=main, got %d", name, n)
		}
		if !strings.Contains(h, `id="content"`) {
			t.Errorf("%s: missing id=content for the main landmark", name)
		}
		if !strings.Contains(h, `<a class="skip-link" href="#content">`) {
			t.Errorf("%s: missing skip link to #content", name)
		}
	}
}

// TestDarkModeToggleKeyboardOperable guards the A2 fix: where the toggle is
// shown it is a focusable button (role + tabindex) and the script handles
// Enter/Space. The toggle lives in the listing/episode meta row; the 404 page
// has no meta row and intentionally omits it (it still honors the saved theme
// via the boot script), so it is excluded.
func TestDarkModeToggleKeyboardOperable(t *testing.T) {
	pages := invariantPages(t)
	for _, name := range []string{"home", "episodes", "episode"} {
		h := pages[name]
		if !strings.Contains(h, `aria-label="Toggle Dark Mode"`) {
			t.Errorf("%s: missing dark-mode toggle", name)
			continue
		}
		if !strings.Contains(h, `role="button"`) || !strings.Contains(h, `tabindex="0"`) {
			t.Errorf("%s: dark-mode toggle not keyboard-focusable (role=button + tabindex=0)", name)
		}
		if !strings.Contains(h, `"keydown"`) || !strings.Contains(h, `e.key==="Enter"`) {
			t.Errorf("%s: dark-mode toggle script missing Enter/Space keydown handling", name)
		}
	}
}

// TestHTMLLangAndTitle guards html-has-lang + document-title.
func TestHTMLLangAndTitle(t *testing.T) {
	for name, h := range invariantPages(t) {
		if !strings.Contains(h, `<html lang="ko">`) {
			t.Errorf("%s: missing <html lang=ko>", name)
		}
		if !strings.Contains(h, "<title>") {
			t.Errorf("%s: missing <title>", name)
		}
	}
}

// TestIframesLazyLoaded guards the P1 fix: every iframe (the GitHub Sponsors
// button in the shared footer) defers loading and carries an accessible title.
func TestIframesLazyLoaded(t *testing.T) {
	for name, h := range invariantPages(t) {
		frames := iframeTagRE.FindAllString(h, -1)
		if len(frames) == 0 {
			t.Errorf("%s: expected the footer sponsors iframe, found none", name)
		}
		for _, tag := range frames {
			if !strings.Contains(tag, `loading="lazy"`) {
				t.Errorf("%s: iframe not lazy-loaded: %s", name, tag)
			}
			if !strings.Contains(tag, "title=") {
				t.Errorf("%s: iframe missing title: %s", name, tag)
			}
		}
	}
}

// TestNoZeroHeightImages guards the P6 fix: no image reserves zero height (which
// reintroduces layout shift when it loads). badges_test pins the exact badge
// heights; this is the page-level backstop.
func TestNoZeroHeightImages(t *testing.T) {
	for name, h := range invariantPages(t) {
		if strings.Contains(h, `height="0"`) {
			t.Errorf("%s: an element reserves height=0 (layout shift risk)", name)
		}
	}
}

// TestLCPPreloadOnlyOnCoverPages guards the P2 fix: the home and 404 pages
// preload the hero cover (their LCP element); the episode and episodes pages
// don't show the cover, so they must not waste a high-priority preload on it.
func TestLCPPreloadOnlyOnCoverPages(t *testing.T) {
	pages := invariantPages(t)
	const preload = `rel="preload" as="image"`
	for _, name := range []string{"home", "404"} {
		if !strings.Contains(pages[name], preload) {
			t.Errorf("%s: missing LCP cover preload", name)
		}
		if !strings.Contains(pages[name], `fetchpriority="high"`) {
			t.Errorf("%s: cover preload missing fetchpriority=high", name)
		}
	}
	for _, name := range []string{"episodes", "episode"} {
		if strings.Contains(pages[name], preload) {
			t.Errorf("%s: unexpected image preload on a page without the cover", name)
		}
	}
}

// TestHeroCoverHasDimensions guards CLS on the cover: the hero <img> declares
// width and height so its box is reserved before the SVG loads.
func TestHeroCoverHasDimensions(t *testing.T) {
	pages := invariantPages(t)
	for _, name := range []string{"home", "404"} {
		h := pages[name]
		for _, tag := range imgTagRE.FindAllString(h, -1) {
			if !strings.Contains(tag, "cover.svg") {
				continue
			}
			if !strings.Contains(tag, "width=") || !strings.Contains(tag, "height=") {
				t.Errorf("%s: cover img missing width/height: %s", name, tag)
			}
		}
	}
}
