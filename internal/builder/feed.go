package builder

import (
	"regexp"
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
// The channel <description> and <generator> were artifacts of the old library
// (the description duplicated the title; the generator read "RSS for Node");
// both now carry accurate RetroTech values. The item <description> is the one
// other deliberate divergence: it is now the HTML podcast apps actually render
// (see descriptionHTML) instead of raw newline-separated text, which those apps
// collapsed into a single paragraph. Everything else still mirrors the old
// output so the subscriber-facing items stay byte-stable.

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

	// The podcast namespace (for <podcast:chapters>) is declared only once an
	// episode actually has chapters, so until then the feed stays byte-identical
	// to the golden the subscribers have been receiving.
	rssOpen := `<rss xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:atom="http://www.w3.org/2005/Atom" version="2.0" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd"`
	if anyChapters(ordered) {
		rssOpen += ` xmlns:podcast="https://podcastindex.org/namespace/1.0"`
	}

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(rssOpen + ">\n")
	b.WriteString("    <channel>\n")
	b.WriteString("        <title>" + cdata(feedTitle) + "</title>\n")
	b.WriteString("        <description>" + cdata(feedDesc) + "</description>\n")
	b.WriteString("        <link>" + site + "</link>\n")
	b.WriteString("        <generator>RetroTech</generator>\n")
	b.WriteString("        <lastBuildDate>" + buildTime.UTC().Format(rfc1123GMT) + "</lastBuildDate>\n")
	b.WriteString(`        <atom:link href="` + site + `/feed.xml" rel="self" type="application/rss+xml"/>` + "\n")
	b.WriteString("        <language>" + cdata("ko") + "</language>\n")
	b.WriteString("        <itunes:owner>\n")
	b.WriteString("            <itunes:name>" + showAuthor + "</itunes:name>\n")
	b.WriteString("            <itunes:email>" + feedOwnerEml + "</itunes:email>\n")
	b.WriteString("        </itunes:owner>\n")
	b.WriteString("        <itunes:author>" + showAuthor + "</itunes:author>\n")
	b.WriteString(`        <itunes:image href="` + site + `/images/cover.jpg"/>` + "\n")
	b.WriteString("        <itunes:explicit>no</itunes:explicit>\n")
	b.WriteString(`        <itunes:category text="Technology">` + "\n")
	b.WriteString("        </itunes:category>\n")

	for _, ep := range ordered {
		url := site + "/episodes/" + ep.ID
		b.WriteString("        <item>\n")
		b.WriteString("            <title>" + cdata(ep.Title) + "</title>\n")
		b.WriteString("            <description>" + cdata(descriptionHTML(feedDescription(ep.Frontmatter))) + "</description>\n")
		b.WriteString("            <link>" + url + "</link>\n")
		b.WriteString(`            <guid isPermaLink="true">` + url + "</guid>\n")
		b.WriteString("            <dc:creator>" + cdata(showAuthor) + "</dc:creator>\n")
		b.WriteString("            <pubDate>" + pubDate(ep.Date) + "</pubDate>\n")
		b.WriteString(`            <enclosure url="` + ep.Enclosure.URL + `" length="` + strconv.FormatInt(ep.Enclosure.Size, 10) + `" type="` + enclosureType(ep.Enclosure.URL) + `"/>` + "\n")
		b.WriteString("            <duration>" + ep.Duration + "</duration>\n")
		b.WriteString("            <itunes:duration>" + ep.Duration + "</itunes:duration>\n")
		b.WriteString("            <itunes:explicit>no</itunes:explicit>\n")
		b.WriteString("            <itunes:author>" + showAuthor + "</itunes:author>\n")
		if len(ep.Chapters) > 0 {
			b.WriteString(`            <podcast:chapters url="` + site + "/" + ChaptersRelPath(ep.ID) + `" type="application/json+chapters"/>` + "\n")
		}
		b.WriteString("        </item>\n")
	}

	b.WriteString("    </channel>\n")
	b.WriteString("</rss>")
	return []byte(b.String())
}

// feedDescription mirrors gen-rss.js: description, with description2 appended
// after a newline when present. Chapters, when declared, are appended as plain
// "MM:SS title" lines — the format Apple Podcasts, Spotify and YouTube
// auto-link as seekable timestamps, which covers apps without Podcasting 2.0
// chapter support.
func feedDescription(fm parser.Frontmatter) string {
	desc := fm.Description
	if fm.Description2 != "" {
		desc = desc + "\n" + fm.Description2
	}
	if len(fm.Chapters) > 0 {
		desc = strings.TrimRight(desc, "\n") + "\n\n" + chapterLines(fm.Chapters)
	}
	return desc
}

// descriptionBlockSep splits the plain description into paragraphs on one or
// more blank lines.
var descriptionBlockSep = regexp.MustCompile(`\n{2,}`)

// bareURL matches an unmarked http(s) link in the description text.
var bareURL = regexp.MustCompile(`https?://[^\s<>"]+`)

// descriptionHTML renders the plain-text feed description as the small HTML
// subset podcast apps accept, because they render <description> as HTML: raw
// newlines collapse into spaces, which is why Apple Podcasts showed the
// description2 block running into the summary as one paragraph. Blank-line
// separated blocks become <p>, the single newlines inside a block become
// <br/>, and bare URLs become real anchors so a link stays clickable in apps
// that do not auto-link. Text is HTML-escaped (the result still ships inside
// CDATA, so the escapes reach the app intact and render as the literal
// characters).
func descriptionHTML(text string) string {
	var b strings.Builder
	for _, block := range descriptionBlockSep.Split(strings.Trim(text, "\n"), -1) {
		if strings.TrimSpace(block) == "" {
			continue
		}
		lines := strings.Split(block, "\n")
		for i, line := range lines {
			lines[i] = escapeAndLink(line)
		}
		b.WriteString("<p>" + strings.Join(lines, "<br/>") + "</p>")
	}
	return b.String()
}

// escapeAndLink HTML-escapes one line and wraps its bare URLs in anchors.
// Trailing sentence punctuation is left outside the anchor so a URL at the end
// of a sentence does not swallow the period.
func escapeAndLink(line string) string {
	var b strings.Builder
	last := 0
	for _, m := range bareURL.FindAllStringIndex(line, -1) {
		start, end := m[0], m[1]
		for end > start && strings.ContainsRune(".,;:!?", rune(line[end-1])) {
			end--
		}
		url := line[start:end]
		b.WriteString(escapeText(line[last:start]))
		b.WriteString(`<a href="` + escapeAttr(url) + `">` + escapeText(url) + `</a>`)
		last = end
	}
	b.WriteString(escapeText(line[last:]))
	return b.String()
}

// escapeText escapes only what a text node must escape. html.EscapeString would
// also turn quotes and apostrophes into entities, which read as literal
// "&#39;" in the apps that strip the tags without decoding entities — common
// enough (and our titles are full of apostrophes) to be worth avoiding.
var escapeText = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace

// escapeAttr additionally escapes the double quote that delimits href.
var escapeAttr = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace

// chapterLines renders chapters one per line as "MM:SS title", keeping the
// source start string verbatim.
func chapterLines(chapters []parser.Chapter) string {
	lines := make([]string, len(chapters))
	for i, ch := range chapters {
		lines[i] = strings.TrimSpace(ch.Start) + " " + strings.TrimSpace(ch.Title)
	}
	return strings.Join(lines, "\n")
}

// anyChapters reports whether at least one episode declares chapters.
func anyChapters(episodes []parser.Episode) bool {
	for _, ep := range episodes {
		if len(ep.Chapters) > 0 {
			return true
		}
	}
	return false
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
