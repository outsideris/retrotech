package editor

import (
	"regexp"
	"strings"

	"github.com/outsideris/retrotech/internal/builder"
)

// The frontmatter description and the body intro have always carried the same
// text (every published episode duplicates it), so the editor treats the intro
// as the single source: for structured forms the stores derive the description
// from the intro at save time and the UI doesn't expose a description field.
// The only difference between the two is markup — the intro may contain
// markdown links, the feed description must not — so deriving means stripping
// link syntax.
//
// markdownLink matches an inline link or image, capturing its text. The URL part
// tolerates one level of nested parentheses (Wikipedia-style URLs).
var markdownLink = regexp.MustCompile(`!?\[([^\]]*)\]\((?:[^()]|\([^()]*\))*\)`)

// deriveDescription builds the frontmatter description from an intro: markdown
// links collapse to their text, and the result carries exactly one trailing
// newline — the block-scalar (clip) convention every existing episode uses,
// which puts the blank line before description2 in the feed. An empty intro
// yields an empty description. The function is idempotent, so re-deriving an
// already-derived description is a no-op.
func deriveDescription(intro string) string {
	text := strings.TrimSpace(markdownLink.ReplaceAllString(intro, "$1"))
	if text == "" {
		return ""
	}
	return text + "\n"
}

// deriveFeedDescription builds the frontmatter feedDescription: the HTML the
// feed ships for this episode, which is what podcast apps actually render (a
// plain description loses every line break there). It is stored in the file
// rather than left to the builder so the markdown shows exactly what
// subscribers receive — description/description2 stay the plain, readable
// source, and this field is derived from them at save time.
//
// It composes the two fields the way the feed does (description2 after a blank
// line) and converts. Chapters are not included: they live in their own
// frontmatter list and the builder appends them, so a chapter edit does not
// have to round-trip through here.
func deriveFeedDescription(description, description2 string) string {
	plain := description
	if description2 != "" {
		plain += "\n" + description2
	}
	html := builder.DescriptionHTML(plain)
	if html == "" {
		return ""
	}
	return html + "\n"
}
