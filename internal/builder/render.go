package builder

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// render.go reproduces the HTML the Nextra blog theme produced, so the reused
// theme stylesheet renders the pages identically. It does NOT reproduce the
// Next.js runtime (the inlined React/JSON/preloads) — dropping that is the
// point of the migration. Parity here means the visible DOM and CSS classes,
// not byte-identity with the framework output.

const (
	siteURL         = "https://retrotech.outsider.dev"
	siteName        = "RetroTech 팟캐스트"
	siteDescription = "기술의 역사를 살펴보는 팟캐스트입니다"
	coverImage      = siteURL + "/images/cover.jpg"
	stylesheetPath  = "/styles.css"
)

// coverPreload preloads the hero cover image (the LCP element on the home and
// 404 pages) so the browser starts fetching it before the body is parsed.
// Episode pages don't show the cover, so they omit it.
const coverPreload = `<link rel="preload" as="image" href="/images/cover.svg" fetchpriority="high"/>`

// md is configured to match the Nextra/remark pipeline the episodes were
// authored against: GFM (so bare URLs autolink, as the show notes rely on) and
// raw-HTML passthrough (the `<div class="refs">` block).
var md = goldmark.New(
	goldmark.WithExtensions(extension.GFM),
	goldmark.WithRendererOptions(gmhtml.WithUnsafe()),
)

// Site carries build-wide values that vary by environment: the copyright year
// shown in the footer, the GA4 measurement id (empty in local/CI builds, so no
// analytics is shipped — only the deploy build sets it), and the content-hashed
// stylesheet href (empty falls back to the unhashed path, e.g. in tests).
type Site struct {
	Year           int
	AnalyticsID    string
	StylesheetHref string
}

// stylesheet returns the stylesheet href for the page, defaulting to the
// unhashed path when the build has not provided a hashed one.
func (s Site) stylesheet() string {
	if s.StylesheetHref != "" {
		return s.StylesheetHref
	}
	return stylesheetPath
}

// BuildEpisodePage renders one episode page.
func BuildEpisodePage(ep parser.Episode, site Site) string {
	title := strings.TrimSpace(ep.Title)
	inner := "<h1>" + html.EscapeString(title) + "</h1>" +
		episodeMeta(ep) +
		renderEpisodeBody(ep)
	return pageShell(title, title+" - RetroTech", inner, site, "")
}

// BuildHomePage renders the site home ("/"): cover, intro, default badges, the
// feedback note, then the episode list.
func BuildHomePage(eps []parser.Episode, site Site) string {
	var b strings.Builder
	b.WriteString("<h1>RetroTech</h1>")
	b.WriteString(listMeta(`<a href="/episodes">Episodes</a><span class="rt-cursor-default dark:rt-text-gray-400 rt-text-gray-600">RetroTech</span>`))
	b.WriteString(`<img alt="RetroTech Cover" fetchpriority="high" width="3000" height="3000" style="width:100%;height:auto" src="/images/cover.svg"/>`)
	b.WriteString("<p>기술별로 과거 어떤 배경에서 기술이 등장하고 발전해 왔는지 또 왜 어떤 기술은 사라졌는지\n기술의 역사를 자세히 설명하는 팟캐스트입니다.</p>")
	b.WriteString("<p>아래 팟캐스트 플랫폼에서 구독해서 듣거나\n" +
		externalLink("https://retrotech.outsider.dev/feed.xml", "피드") + "를 직접 등록해서 들을  수 있습니다.</p>")
	b.WriteString("<hr/>")
	b.WriteString(RenderBadges(parser.Badges{}))
	b.WriteString("<hr/>")
	b.WriteString("<p>내용 중 잘못된 부분이나 다뤄줬으면 하는 주제는\n" +
		externalLink("https://github.com/outsideris/retrotech/issues", "GitHub Issues") + "나\n" +
		externalLink("https://twitter.com/outsideris", "Twitter") + "에서 알려주세요.</p>")
	b.WriteString("<hr/>")
	b.WriteString("<h2>에피소드</h2>")
	b.WriteString(renderPostList(eps))
	return pageShell(siteName, siteName, b.String(), site, coverPreload)
}

// BuildEpisodesPage renders the "/episodes" listing. The nav row mirrors the
// theme: the current page ("Episodes") as plain text and a link back to the
// home page ("RetroTech").
func BuildEpisodesPage(eps []parser.Episode, site Site) string {
	inner := "<h1>Episodes</h1>" +
		listMeta(`<span class="rt-cursor-default dark:rt-text-gray-400 rt-text-gray-600">Episodes</span><a href="/">RetroTech</a>`) +
		renderPostList(eps)
	return pageShell("Episodes - RetroTech", "Episodes - RetroTech", inner, site, "")
}

// Build404Page renders the 404 page: the RetroTech cover (as on the home page,
// here linking home so a lost visitor has a way back) above the not-found
// message.
func Build404Page(site Site) string {
	inner := `<a href="/"><img alt="RetroTech Cover" width="3000" height="3000" style="width:100%;height:auto" src="/images/cover.svg"/></a>` +
		"<h1>404: Page Not Found</h1>"
	return pageShell("404: Page Not Found - RetroTech", "404: Page Not Found - RetroTech", inner, site, coverPreload)
}

// renderPostList renders the list of episodes shown on the home and episodes
// pages (newest-first).
func renderPostList(eps []parser.Episode) string {
	var b strings.Builder
	for _, ep := range eps {
		url := "/episodes/" + ep.ID
		title := strings.TrimSpace(ep.Title)
		b.WriteString(`<div class="post-item">`)
		b.WriteString(`<h3><a class="!rt-no-underline" href="` + url + `">` + html.EscapeString(title) + `</a></h3>`)
		b.WriteString(`<p class="rt-mb-2 dark:rt-text-gray-400 rt-text-gray-600">` +
			html.EscapeString(strings.TrimRight(ep.Description, "\n")) +
			`<a class="post-item-more rt-ml-2" href="` + url + `">Read More →</a></p>`)
		b.WriteString(`<time class="rt-text-sm dark:rt-text-gray-400 rt-text-gray-600" dateTime="` +
			isoDate(ep.Date) + `">` + displayDate(ep.Date) + `</time>`)
		b.WriteString(`</div>`)
	}
	return b.String()
}

// renderEpisodeBody renders an episode's markdown body to HTML and applies the
// Nextra-equivalent transforms: external links open in a new tab, markdown
// headings gain permalink anchors, and the <!--badges--> marker is replaced
// with the subscription badges.
func renderEpisodeBody(ep parser.Episode) string {
	var buf bytes.Buffer
	if err := md.Convert([]byte(ep.Body), &buf); err != nil {
		return ""
	}
	out := buf.String()
	out = wrapReferences(out)
	out = decorateExternalLinks(out)
	out = decorateHeadings(out)
	out = strings.ReplaceAll(out, "<!--badges-->", RenderBadges(ep.Badges))
	return out
}

var (
	// External (http/https) anchors only — internal links keep their plain form.
	externalAnchorRE = regexp.MustCompile(`<a href="(https?://[^"]+)">(.*?)</a>`)
	headingRE        = regexp.MustCompile(`(?s)<h([2-6])>(.*?)</h[2-6]>`)
	tagRE            = regexp.MustCompile(`<[^>]+>`)
	// The list following a "#### 레퍼런스:" heading is the references block. The
	// builder wraps it in <div class="refs"> so the show notes stay plain
	// markdown (no raw HTML), while the .refs styling (smaller, denser) is the
	// same as before. Runs before decorateHeadings, on the bare <h4>.
	referencesRE = regexp.MustCompile(`(?s)(<h4>레퍼런스:</h4>\s*)(<ul>.*?</ul>)`)
)

// wrapReferences wraps the reference list under the "레퍼런스:" heading in
// <div class="refs">, reproducing the markup the show notes used to carry
// inline.
func wrapReferences(s string) string {
	return referencesRE.ReplaceAllString(s, `${1}<div class="refs">${2}</div>`)
}

// decorateExternalLinks rewrites external anchors to open in a new tab with the
// screen-reader hint, matching Nextra's link handling.
func decorateExternalLinks(s string) string {
	return externalAnchorRE.ReplaceAllString(s,
		`<a target="_blank" rel="noreferrer" href="$1">$2<span class="rt-sr-only rt-select-none"> (opens in a new tab)</span></a>`)
}

// externalLink builds a single decorated external link (used in the hand-written
// home copy).
func externalLink(href, text string) string {
	return `<a target="_blank" rel="noreferrer" href="` + html.EscapeString(href) + `">` +
		html.EscapeString(text) + `<span class="rt-sr-only rt-select-none"> (opens in a new tab)</span></a>`
}

// decorateHeadings adds Nextra's permalink-anchor structure to markdown
// headings (h2–h6).
func decorateHeadings(s string) string {
	return headingRE.ReplaceAllStringFunc(s, func(m string) string {
		sub := headingRE.FindStringSubmatch(m)
		level, inner := sub[1], sub[2]
		slug := slugify(tagRE.ReplaceAllString(inner, ""))
		return "<h" + level + ` class="subheading-h` + level + `">` + inner +
			`<span class="rt-absolute -rt-mt-7" id="` + slug + `"></span>` +
			`<a href="#` + slug + `" class="subheading-anchor" aria-label="Permalink for this section"></a>` +
			"</h" + level + ">"
	})
}

// slugify mirrors github-slugger for the heading text used in the show notes:
// lowercased, punctuation dropped, spaces to hyphens, unicode letters kept.
func slugify(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == ' ':
			b.WriteByte('-')
		case r == '-' || r == '_':
			b.WriteRune(r)
		case isAlnum(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isAlnum(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r > 127
}
