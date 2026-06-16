package builder

import (
	"html"
	"strconv"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

// Subscription badge defaults — the show-level channel links the old
// components/Badges.tsx baked in as prop defaults. Episodes override them with
// per-episode deep links via frontmatter `badges:`.
const (
	defaultApple   = "https://podcasts.apple.com/kr/podcast/retrotech-%ED%8C%9F%EC%BA%90%EC%8A%A4%ED%8A%B8/id1698903712"
	defaultYouTube = "https://www.youtube.com/playlist?list=PLEHf_UYxvkp9HCnP3UIZhEss_Yo4XuUDX"
	defaultSpotify = "https://open.spotify.com/show/3nSplj43Rd86snTrsEHdTI"
	defaultRSS     = "/feed.xml"
)

// RenderBadges returns the subscription badges block, reproducing the markup
// the old components/Badges.tsx rendered: Apple, YouTube and Spotify are always
// shown; when Google is set its badge appears, otherwise the RSS badge does.
// Empty fields fall back to the show-level channel links.
func RenderBadges(b parser.Badges) string {
	var sb strings.Builder
	sb.WriteString(`<div class="badges">`)
	sb.WriteString(badgeLink(orDefault(b.Apple, defaultApple), "", "/badges/apple.svg", "Listen on Apple Podcasts", "badge", 300))
	sb.WriteString(badgeLink(orDefault(b.YouTube, defaultYouTube), "youtube", "/badges/youtube.svg", "Available on YouTube", "badge youtube", 240))
	sb.WriteString(badgeLink(orDefault(b.Spotify, defaultSpotify), "", "/badges/spotify.svg", "Listen on Spotify", "badge spotify", 300))
	if b.Google != "" {
		sb.WriteString(badgeLink(b.Google, "", "/badges/google.svg", "Listen on Google Podcasts", "badge", 300))
	} else {
		sb.WriteString(badgeLink(orDefault(b.RSS, defaultRSS), "", "/badges/rss.svg", "Get the RSS Feed", "badge", 300))
	}
	sb.WriteString(`</div>`)
	return sb.String()
}

// badgeLink renders one `<a><img></a>` badge. linkClass is empty for every
// badge except YouTube (which carries a "youtube" class on the anchor for its
// wider spacing).
func badgeLink(href, linkClass, src, alt, imgClass string, width int) string {
	a := "<a"
	if linkClass != "" {
		a += ` class="` + linkClass + `"`
	}
	a += ` href="` + html.EscapeString(href) + `">`
	img := `<img alt="` + html.EscapeString(alt) + `" loading="lazy" width="` +
		strconv.Itoa(width) + `" height="0" decoding="async" class="` + imgClass + `" src="` + src + `"/>`
	return a + img + "</a>"
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
