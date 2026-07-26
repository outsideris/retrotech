// Package editor is the backend for the RetroTech episode management desktop
// app. It turns episode markdown files into a structured form the UI can edit
// without the author touching markdown, and composes that form back into a
// markdown file. The Electron shell (desktop/) and the cmd/app sidecar server
// own the window and HTTP plumbing; this package owns the episode data.
//
// The single hard contract is the RSS feed: builder.BuildFeed reads only the
// frontmatter (title/description/description2/author/date/enclosure/duration),
// so a composed file that re-parses to the same parser.Frontmatter values keeps
// the subscriber-facing feed byte-identical regardless of YAML style. compose.go
// guarantees that value fidelity; form.go guarantees the body round-trips
// exactly for episodes it can structure, and falls back to a verbatim raw body
// for anything it cannot, so no content is ever lost.
package editor

import (
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

// badgesMarker is the literal the site builder replaces with the subscription
// badge block; every episode body carries it. refsHeading introduces the
// reference link list. Both are managed by the composer so the author edits
// structured fields, never these markers.
const (
	badgesMarker = "<!--badges-->"
	refsHeading  = "## 레퍼런스:"
	refIndent    = "    " // one nesting level in a reference list (4 spaces)
)

// Reference is one item in an episode's reference list. URL is empty for a
// plain-text bullet (e.g. a section label like "* Svelte" that groups nested
// links). Indent is the nesting depth (0 = top level); children sit one level
// deeper under a plain-text parent.
type Reference struct {
	Text   string `json:"text"`
	URL    string `json:"url"`
	Indent int    `json:"indent"`
}

// EpisodeForm is the editable, JSON-friendly view of an episode. The frontmatter
// fields mirror parser.Frontmatter one-to-one. The body is split into Intro
// (text before the badges marker), References (the reference list), and Extra
// (anything after the list, e.g. a "## 배경음악" credit) — all losslessly. When
// the body does not match that shape, Structured is false and RawBody holds the
// whole body verbatim so the UI can still edit it as markdown.
type EpisodeForm struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Date          string        `json:"date"`
	Description   string        `json:"description"`
	Description2  string        `json:"description2"`
	EnclosureURL  string        `json:"enclosureUrl"`
	EnclosureSize int64         `json:"enclosureSize"`
	Duration      string        `json:"duration"`
	Badges        parser.Badges `json:"badges"`

	Intro      string      `json:"intro"`
	References []Reference `json:"references"`
	Extra      string      `json:"extra"`

	RawBody    string `json:"rawBody"`
	Structured bool   `json:"structured"`
}

// EpisodeSummary is the lightweight episode entry shown in the list sidebar.
type EpisodeSummary struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Duration string `json:"duration"`
}

// EpisodeToForm builds an editable form from a parsed episode. The frontmatter
// copies straight across; the body is structured when possible and otherwise
// preserved verbatim in RawBody.
func EpisodeToForm(ep parser.Episode) EpisodeForm {
	f := EpisodeForm{
		ID:            ep.ID,
		Title:         ep.Title,
		Date:          ep.Date,
		Description:   ep.Description,
		Description2:  ep.Description2,
		EnclosureURL:  ep.Enclosure.URL,
		EnclosureSize: ep.Enclosure.Size,
		Duration:      ep.Duration,
		Badges:        ep.Badges,
	}

	if intro, refs, extra, ok := parseBody(ep.Body); ok {
		f.Structured = true
		f.Intro = intro
		f.References = refs
		f.Extra = extra
	} else {
		f.Structured = false
		f.RawBody = ep.Body
	}
	return f
}

// summaryOf builds a list entry from a parsed episode.
func summaryOf(ep parser.Episode) EpisodeSummary {
	return EpisodeSummary{
		ID:       ep.ID,
		Title:    strings.TrimSpace(ep.Title),
		Date:     ep.Date,
		Duration: ep.Duration,
	}
}

// parseBody splits a trimmed episode body into intro, references and extra.
// It returns ok=false (telling the caller to keep the body verbatim) whenever
// the body deviates from the expected shape, so structuring never loses or
// reshapes content it does not fully understand.
//
// Expected shape:
//
//	<intro paragraphs>
//
//	<!--badges-->
//
//	## 레퍼런스:
//
//	* [text](url)            (optionally nested, optionally plain text)
//	...
//
//	## 배경음악               (optional; everything from here on is Extra)
//	...
func parseBody(body string) (intro string, refs []Reference, extra string, ok bool) {
	idx := strings.Index(body, badgesMarker)
	if idx < 0 {
		return "", nil, "", false
	}
	intro = strings.TrimRight(body[:idx], " \t\n")
	after := strings.TrimLeft(body[idx+len(badgesMarker):], " \t\n")

	// Badges marker with nothing after it (e.g. the "Breaks" filler episode).
	if after == "" {
		return intro, nil, "", true
	}

	lines := strings.Split(after, "\n")
	if strings.TrimRight(lines[0], " \t") != refsHeading {
		// Content after the marker that isn't the reference section — leave it
		// to the raw-body fallback rather than guess.
		return "", nil, "", false
	}

	// Split the post-heading lines into the reference block and an optional
	// trailing section (the first "## " heading starts Extra).
	var refLines, extraLines []string
	inExtra := false
	for _, ln := range lines[1:] {
		if !inExtra && strings.HasPrefix(ln, "## ") {
			inExtra = true
		}
		if inExtra {
			extraLines = append(extraLines, ln)
		} else {
			refLines = append(refLines, ln)
		}
	}

	parsed, refsOK := parseReferences(refLines)
	if !refsOK || len(parsed) == 0 {
		// An empty or non-flat reference list — keep the body verbatim so a
		// re-save can't drop the heading or flatten nested items.
		return "", nil, "", false
	}

	return intro, parsed, strings.TrimSpace(strings.Join(extraLines, "\n")), true
}

// parseReferences converts the lines of a reference block into structured
// items. It accepts only a clean list of "* " bullets indented in multiples of
// four spaces, each either "[text](url)" or plain text. Any other content
// (blank line mid-list, non-bullet line, odd indentation) returns ok=false,
// routing the whole episode to the verbatim raw-body fallback.
func parseReferences(lines []string) (refs []Reference, ok bool) {
	// Trim the blank lines that frame the block (after the heading, before the
	// next section); they are re-added by the composer.
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	lines = lines[start:end]

	for _, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			return nil, false // a blank line inside the list — not a flat list
		}
		rest := strings.TrimLeft(ln, " ")
		lead := len(ln) - len(rest)
		if lead%4 != 0 || !strings.HasPrefix(rest, "* ") {
			return nil, false
		}
		text, url := parseLinkItem(rest[len("* "):])
		refs = append(refs, Reference{Text: text, URL: url, Indent: lead / 4})
	}
	return refs, true
}

// parseLinkItem splits a bullet's content into text and URL. A markdown link
// "[text](url)" yields both; anything else is treated as plain text with an
// empty URL. The split is the exact inverse of composeReferences, so
// re-composing always reproduces the original bullet byte-for-byte (URLs may
// themselves contain parentheses, so the URL runs to the last ")").
func parseLinkItem(item string) (text, url string) {
	if strings.HasPrefix(item, "[") && strings.HasSuffix(item, ")") {
		if i := strings.Index(item, "]("); i > 0 {
			return item[1:i], item[i+len("](") : len(item)-1]
		}
	}
	return item, ""
}
