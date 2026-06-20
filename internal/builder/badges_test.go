package builder

import (
	"strings"
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

func TestRenderBadgesDefaultsShowRSS(t *testing.T) {
	got := RenderBadges(parser.Badges{})
	for _, want := range []string{
		`<div class="badges">`,
		`href="` + defaultApple + `"`,
		`<a class="youtube" href="` + defaultYouTube + `"`,
		`href="` + defaultSpotify + `"`,
		`src="/badges/apple.svg"`,
		`src="/badges/youtube.svg"`,
		`src="/badges/spotify.svg"`,
		`src="/badges/rss.svg"`, // no google → RSS badge
		`href="/feed.xml"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "google.svg") {
		t.Errorf("RSS badge expected, found Google badge:\n%s", got)
	}
}

func TestRenderBadgesGoogleReplacesRSS(t *testing.T) {
	got := RenderBadges(parser.Badges{Google: "https://podcasts.google.com/feed/x"})
	if !strings.Contains(got, "src=\"/badges/google.svg\"") {
		t.Errorf("expected Google badge:\n%s", got)
	}
	if strings.Contains(got, "rss.svg") {
		t.Errorf("RSS badge should be absent when Google is set:\n%s", got)
	}
}

// Each badge must declare a non-zero height matching its SVG aspect ratio so
// the lazy image reserves its box up front (no layout shift / CLS when it
// loads). The width÷aspect heights: apple/spotify 73 (≈4.12:1 at w=300),
// youtube 78 (w=240), rss 75 (w=300). A regression to height="0" reintroduces
// the shift, so assert the exact values.
func TestRenderBadgesReserveHeightToAvoidCLS(t *testing.T) {
	got := RenderBadges(parser.Badges{})
	if strings.Contains(got, `height="0"`) {
		t.Errorf("badge height=0 reintroduces layout shift:\n%s", got)
	}
	for _, want := range []string{
		`width="300" height="73" decoding="async" class="badge" src="/badges/apple.svg"`,
		`width="240" height="78" decoding="async" class="badge youtube" src="/badges/youtube.svg"`,
		`width="300" height="73" decoding="async" class="badge spotify" src="/badges/spotify.svg"`,
		`width="300" height="75" decoding="async" class="badge" src="/badges/rss.svg"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing reserved-height badge %q in:\n%s", want, got)
		}
	}
}

func TestRenderBadgesEpisodeDeepLinksAndEscaping(t *testing.T) {
	got := RenderBadges(parser.Badges{
		Apple:   "https://podcasts.apple.com/kr/podcast/x/id1?i=2",
		YouTube: "https://www.youtube.com/watch?v=abc&list=def",
		Spotify: "https://open.spotify.com/episode/z",
	})
	if !strings.Contains(got, "id1?i=2") {
		t.Errorf("apple deep link missing:\n%s", got)
	}
	// The ampersand in the YouTube URL must be HTML-escaped in the href.
	if !strings.Contains(got, "v=abc&amp;list=def") {
		t.Errorf("youtube href not escaped:\n%s", got)
	}
}
