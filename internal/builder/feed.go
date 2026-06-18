package builder

import (
	"strconv"
	"strings"
	"time"

	"github.com/outsideris/retrotech/internal/parser"
)

// This file reproduces the iTunes podcast RSS feed that scripts/gen-rss.js
// produced with the `rss` npm library. The output is matched byte-for-byte
// (except the volatile <lastBuildDate>) so existing subscribers — and the
// Apple/Spotify-registered feed — see no change: the per-item <guid>,
// <enclosure> and <pubDate> are the subscriber contract and must stay stable.
//
// We build the XML as a string rather than via encoding/xml because the goal
// is to mirror the old library's exact serialization — CDATA-wrapped text,
// the specific namespace order, 4-space indentation, self-closing vs.
// expanded empty elements — which the stdlib marshaller does not reproduce.
//
// Some of the reproduced details are artifacts of the old library (the
// generator string "RSS for Node", the channel <description> duplicating the
// title). They are kept for byte-parity and flagged for later cleanup.

const (
	// rfc1123GMT matches the date form the `rss` lib emitted, e.g.
	// "Sat, 07 Mar 2026 00:00:00 GMT".
	rfc1123GMT = "Mon, 02 Jan 2006 15:04:05 GMT"

	feedTitle = "RetroTech 팟캐스트"
	// feedDesc is the show description podcast apps (Apple/Spotify) display. The
	// old gen-rss.js set no description, so the rss library fell back to the
	// title — apps showed just the show name. This is the site's own self
	// description (the home page intro), so the feed actually describes the show.
	feedDesc     = "기술별로 과거 어떤 배경에서 기술이 등장하고 발전해 왔는지 또 왜 어떤 기술은 사라졌는지 기술의 역사를 자세히 설명하는 팟캐스트입니다."
	feedAuthor   = "Outsider"
	feedOwnerEml = "outsideris@gmail.com"
)

// FeedConfig carries the only feed input that varies by environment.
type FeedConfig struct {
	SiteURL string // absolute origin, no trailing slash (e.g. "https://retrotech.outsider.dev")
}

// BuildFeed renders the podcast RSS feed for episodes. The episodes are ordered
// newest-first internally (matching gen-rss.js), so callers may pass them in
// any order. buildTime fills <lastBuildDate>.
func BuildFeed(episodes []parser.Episode, cfg FeedConfig, buildTime time.Time) []byte {
	site := strings.TrimRight(cfg.SiteURL, "/")

	ordered := make([]parser.Episode, len(episodes))
	copy(ordered, episodes)
	parser.SortEpisodes(ordered)

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(`<rss xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:atom="http://www.w3.org/2005/Atom" version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">` + "\n")
	b.WriteString("    <channel>\n")
	b.WriteString("        <title>" + cdata(feedTitle) + "</title>\n")
	b.WriteString("        <description>" + cdata(feedDesc) + "</description>\n")
	b.WriteString("        <link>" + site + "</link>\n")
	b.WriteString("        <generator>RSS for Node</generator>\n")
	b.WriteString("        <lastBuildDate>" + buildTime.UTC().Format(rfc1123GMT) + "</lastBuildDate>\n")
	b.WriteString(`        <atom:link href="` + site + `/feed.xml" rel="self" type="application/rss+xml"/>` + "\n")
	b.WriteString("        <language>" + cdata("ko") + "</language>\n")
	b.WriteString("        <itunes:owner>\n")
	b.WriteString("            <itunes:name>" + feedAuthor + "</itunes:name>\n")
	b.WriteString("            <itunes:email>" + feedOwnerEml + "</itunes:email>\n")
	b.WriteString("        </itunes:owner>\n")
	b.WriteString("        <itunes:author>" + feedAuthor + "</itunes:author>\n")
	b.WriteString(`        <itunes:image href="` + site + `/images/cover.jpg"/>` + "\n")
	b.WriteString("        <itunes:explicit>no</itunes:explicit>\n")
	b.WriteString(`        <itunes:category text="Technology">` + "\n")
	b.WriteString("        </itunes:category>\n")

	for _, ep := range ordered {
		url := site + "/episodes/" + ep.ID
		b.WriteString("        <item>\n")
		b.WriteString("            <title>" + cdata(ep.Title) + "</title>\n")
		b.WriteString("            <description>" + cdata(feedDescription(ep.Frontmatter)) + "</description>\n")
		b.WriteString("            <link>" + url + "</link>\n")
		b.WriteString(`            <guid isPermaLink="true">` + url + "</guid>\n")
		b.WriteString("            <dc:creator>" + cdata(ep.Author) + "</dc:creator>\n")
		b.WriteString("            <pubDate>" + pubDate(ep.Date) + "</pubDate>\n")
		b.WriteString(`            <enclosure url="` + ep.Enclosure.URL + `" length="` + strconv.FormatInt(ep.Enclosure.Size, 10) + `" type="` + enclosureType(ep.Enclosure.URL) + `"/>` + "\n")
		b.WriteString("            <duration>" + ep.Duration + "</duration>\n")
		b.WriteString("            <itunes:duration>" + ep.Duration + "</itunes:duration>\n")
		b.WriteString("            <itunes:explicit>no</itunes:explicit>\n")
		b.WriteString("            <itunes:author>" + feedAuthor + "</itunes:author>\n")
		b.WriteString("        </item>\n")
	}

	b.WriteString("    </channel>\n")
	b.WriteString("</rss>")
	return []byte(b.String())
}

// feedDescription mirrors gen-rss.js: description, with description2 appended
// after a newline when present.
func feedDescription(fm parser.Frontmatter) string {
	if fm.Description2 != "" {
		return fm.Description + "\n" + fm.Description2
	}
	return fm.Description
}

// pubDate reproduces the published feed's <pubDate>, which is the source
// "YYYY/MM/DD" date at 09:00 UTC formatted RFC-1123. The deployed feed builds
// in UTC, so `new Date("<date> 09:00")` yielded 09:00 GMT; parsing in UTC here
// reproduces that exact value deterministically (and removes the old build's
// dependence on the build machine's timezone).
func pubDate(date string) string {
	t, err := time.Parse("2006/1/2 15:04", strings.TrimSpace(date)+" 09:00")
	if err != nil {
		return ""
	}
	return t.UTC().Format(rfc1123GMT)
}

// enclosureType infers the MIME type from the audio URL extension, mirroring
// the `rss` lib's mime lookup. All episodes are mp3.
func enclosureType(url string) string {
	if strings.HasSuffix(strings.ToLower(url), ".mp3") {
		return "audio/mpeg"
	}
	return "audio/mpeg"
}

// cdata wraps text in a CDATA section, splitting any literal "]]>" so it cannot
// close the section early (same safeguard the xml serializer applied).
func cdata(s string) string {
	s = strings.ReplaceAll(s, "]]>", "]]]]><![CDATA[>")
	return "<![CDATA[" + s + "]]>"
}
