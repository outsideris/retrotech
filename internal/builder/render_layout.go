package builder

import (
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/outsideris/retrotech/internal/parser"
)

// pageShell wraps an article's inner HTML in the full document: the head, the
// dark-mode bootstrap, the prose article container (with the shared footer
// inside it, as the Nextra theme placed it), and the dark-mode toggle handler.
// It reproduces the Nextra page chrome minus the Next.js runtime.
func pageShell(title, ogTitle, articleInner string, site Site) string {
	return "<!DOCTYPE html><html lang=\"ko\"><head>" +
		darkModeInit +
		headHTML(title, ogTitle, site) +
		"</head><body><div id=\"app\">" +
		`<article class="rt-container rt-prose max-md:rt-prose-sm dark:rt-prose-dark" dir="ltr">` +
		articleInner +
		footerHTML(site) +
		"</article>" +
		"</div>" +
		darkModeToggleScript +
		"</body></html>"
}

// headHTML builds the <head> contents. title/ogTitle vary per page; everything
// else is constant. The <!-- @analytics --> marker is replaced with the GA
// snippet only when site.AnalyticsID is set (deploy builds).
func headHTML(title, ogTitle string, site Site) string {
	h := `<meta charset="utf-8"/>` +
		`<meta name="viewport" content="width=device-width"/>` +
		`<link rel="alternate" type="application/rss+xml" title="RSS" href="/feed.xml"/>` +
		"<title>" + html.EscapeString(title) + "</title>" +
		`<meta property="og:title" content="` + html.EscapeString(ogTitle) + `"/>` +
		`<meta name="twitter:title" content="` + html.EscapeString(ogTitle) + `"/>` +
		`<meta name="robots" content="follow, index"/>` +
		`<meta name="description" content="` + siteDescription + `"/>` +
		`<meta property="og:site_name" content="` + siteName + `"/>` +
		`<meta property="og:description" content="` + siteDescription + `"/>` +
		`<meta property="og:image" content="` + coverImage + `"/>` +
		`<meta name="twitter:card" content="summary"/>` +
		`<meta name="twitter:site" content="@outsideris"/>` +
		`<meta name="twitter:creator" content="@outsideris"/>` +
		`<meta name="twitter:description" content="` + siteDescription + `"/>` +
		`<meta name="twitter:image" content="` + coverImage + `"/>` +
		`<link rel="icon" href="/favicon.ico" sizes="any"/>` +
		`<link rel="icon" type="image/png" sizes="32x32" href="/favicon-32x32.png"/>` +
		`<link rel="icon" type="image/png" sizes="16x16" href="/favicon-16x16.png"/>` +
		`<link rel="apple-touch-icon" sizes="180x180" href="/apple-touch-icon.png"/>` +
		`<link rel="manifest" href="/site.webmanifest"/>` +
		`<link rel="stylesheet" href="` + site.stylesheet() + `"/>` +
		footerStyle +
		themeToggleStyle +
		analyticsTag(site.AnalyticsID)
	return h
}

// episodeMeta is the byline row beneath an episode's <h1>: author, date, a Back
// link and the dark-mode toggle.
func episodeMeta(ep parser.Episode) string {
	return `<div class="rt-mb-8 rt-flex rt-gap-3 rt-items-center">` +
		`<div class="rt-grow dark:rt-text-gray-400 rt-text-gray-600">` +
		`<div class="rt-not-prose rt-flex rt-flex-wrap rt-items-center rt-gap-1">` +
		html.EscapeString(ep.Author) + `,<time dateTime="` + isoDate(ep.Date) + `">` + displayDate(ep.Date) + `</time>` +
		`</div></div>` +
		`<div class="rt-flex rt-items-center rt-gap-3 print:rt-hidden">` +
		`<a href="/episodes">Back</a>` + darkToggle +
		`</div></div>`
}

// listMeta is the nav + toggle row beneath a listing page's <h1>. navInner is
// the page-specific nav links.
func listMeta(navInner string) string {
	return `<div class="rt-mb-8 rt-flex rt-items-center rt-gap-3">` +
		`<div class="rt-flex rt-grow rt-flex-wrap rt-items-center rt-justify-end rt-gap-3">` +
		navInner +
		`</div>` + darkToggle + `</div>`
}

// footerHTML is the site footer (host profile, social links, RSS), shared by
// every page. year is the build year shown in the copyright line.
func footerHTML(site Site) string {
	return `<footer>` +
		`<iframe src="https://github.com/sponsors/outsideris/button" title="Sponsor outsideris" height="32" width="114"></iframe>` +
		`<h3>Host:</h3>` +
		`<div>` +
		`<img src="/images/outsider.webp" alt="Outsider" width="120" height="120" class="profile"/>` +
		`<strong>Outsider</strong><br/>` +
		iconTwitter + ` <a href="https://twitter.com/outsideris">outsideris</a><br/>` +
		iconGitHub + ` <a href="https://github.com/outsideris">outsideris</a><br/>` +
		iconBlog + ` <a href="https://blog.outsider.ne.kr/">blog.outsider.ne.kr</a>` +
		`</div>` +
		`<small><time>` + strconv.Itoa(site.Year) + `</time> © Outsider.` +
		`<a href="/feed.xml" aria-label="RSS 피드">` + iconRSS + `</a></small>` +
		`</footer>`
}

// analyticsTag returns the GA4 snippet when id is set, otherwise nothing —
// keeping local/CI/preview builds analytics-free.
func analyticsTag(id string) string {
	if id == "" {
		return ""
	}
	return `<script async src="https://www.googletagmanager.com/gtag/js?id=` + id + `"></script>` +
		`<script>window.dataLayer=window.dataLayer||[];function gtag(){dataLayer.push(arguments);}` +
		`gtag('js',new Date());gtag('config','` + id + `');</script>`
}

// isoDate renders the episode date as the machine-readable datetime attribute:
// the date at 00:00 UTC, e.g. "2026-03-07T00:00:00.000Z".
func isoDate(date string) string {
	t, err := time.Parse("2006/1/2", strings.TrimSpace(date))
	if err != nil {
		return ""
	}
	return t.UTC().Format("2006-01-02") + "T00:00:00.000Z"
}

// displayDate renders the human date shown to readers, matching JS
// Date.toDateString(), e.g. "Sat Mar 07 2026".
func displayDate(date string) string {
	t, err := time.Parse("2006/1/2", strings.TrimSpace(date))
	if err != nil {
		return ""
	}
	return t.Format("Mon Jan 02 2006")
}
