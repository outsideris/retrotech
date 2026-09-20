package editor

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

// ComposeFile renders an EpisodeForm back into the bytes of an episode markdown
// file: the YAML frontmatter, the "---" fence, then the body. The output is
// canonical (fixed key order, literal block scalars) rather than a byte copy of
// whatever style the source used — but it always re-parses to the same
// parser.Frontmatter values, so builder.BuildFeed stays byte-identical (see the
// round-trip tests). The body round-trips exactly for structured episodes.
func ComposeFile(f EpisodeForm) []byte {
	return []byte("---\n" + composeFrontmatter(f) + "---\n\n" + composeBody(f) + "\n")
}

// composeFrontmatter serializes the frontmatter in parser.Frontmatter field
// order. It does not use yaml.Marshal: the marshaller reorders keys, picks its
// own scalar styles and would not reproduce the trailing-newline-significant
// text fields the feed embeds verbatim.
func composeFrontmatter(f EpisodeForm) string {
	var b strings.Builder

	// title/description/description2 carry trailing newlines that the feed
	// reproduces verbatim, so they go out as literal block scalars whose
	// chomping indicator encodes the exact trailing-newline count.
	b.WriteString(blockScalar("title", f.Title))
	b.WriteString("date: " + scalar(f.Date) + "\n")
	b.WriteString(blockScalar("description", f.Description))
	if f.Description2 != "" {
		b.WriteString(blockScalar("description2", f.Description2))
	}
	// The HTML the feed ships. Omitted when empty so an episode written before
	// the field existed stays byte-identical on a round trip.
	if f.FeedDescription != "" {
		b.WriteString(blockScalar("feedDescription", f.FeedDescription))
	}
	// No author: the host is always the same, so it's hard-coded in the
	// builder (byline + feed creator), not stored per-episode.
	b.WriteString("enclosure:\n")
	b.WriteString("  url: " + scalar(f.EnclosureURL) + "\n")
	b.WriteString("  size: " + strconv.FormatInt(f.EnclosureSize, 10) + "\n")
	// Always double-quote duration: bare "17:27" is parsed by YAML as the
	// sexagesimal number 1047, corrupting the feed's <duration>.
	b.WriteString("duration: " + doubleQuote(f.Duration) + "\n")

	if fields := nonEmptyBadges(f.Badges); len(fields) > 0 {
		b.WriteString("badges:\n")
		for _, kv := range fields {
			b.WriteString("  " + kv.key + ": " + doubleQuote(kv.val) + "\n")
		}
	}

	// Chapters come last, matching parser.Frontmatter field order. Omitted when
	// empty so an episode with no chapters keeps its exact frontmatter on a
	// round trip. start is always double-quoted for the same reason duration
	// is: bare "02:41" is sexagesimal YAML and would parse as 161.
	if len(f.Chapters) > 0 {
		b.WriteString("chapters:\n")
		for _, ch := range f.Chapters {
			b.WriteString("  - start: " + doubleQuote(ch.Start) + "\n")
			b.WriteString("    title: " + scalar(ch.Title) + "\n")
		}
	}
	return b.String()
}

// composeBody reassembles the episode body. For an unstructured form it returns
// the verbatim raw body; otherwise it rebuilds intro + badges marker + optional
// reference list + optional trailing section, omitting sections that are empty
// (a "Breaks" episode has no reference list, an episode with no background-music
// credit has no trailing section), so it reproduces the source body exactly.
func composeBody(f EpisodeForm) string {
	if !f.Structured {
		return f.RawBody
	}
	var b strings.Builder
	b.WriteString(f.Intro)
	b.WriteString("\n\n" + badgesMarker)
	if len(f.References) > 0 {
		b.WriteString("\n\n" + refsHeading + "\n\n" + composeReferences(f.References))
	}
	if extra := strings.TrimSpace(f.Extra); extra != "" {
		b.WriteString("\n\n" + extra)
	}
	return b.String()
}

// composeReferences renders the reference list. Each item is "* " (indented by
// its nesting depth) followed by a markdown link, or just the text when the
// item has no URL. This is the exact inverse of parseLinkItem.
func composeReferences(refs []Reference) string {
	lines := make([]string, len(refs))
	for i, r := range refs {
		prefix := strings.Repeat(refIndent, r.Indent) + "* "
		if r.URL != "" {
			lines[i] = prefix + "[" + r.Text + "](" + r.URL + ")"
		} else {
			lines[i] = prefix + r.Text
		}
	}
	return strings.Join(lines, "\n")
}

// blockScalar emits "key: |<chomp>" followed by the value indented four spaces,
// choosing the chomping indicator so the parsed value keeps its exact trailing
// newlines:
//
//	no trailing newline   -> "|-" (strip)
//	one trailing newline  -> "|"  (clip, the default)
//	N>=2 trailing newlines -> "|+" (keep) plus N-1 explicit blank lines
//
// A literal block also sidesteps YAML quoting hazards in the text (a title may
// contain ": ", a description may span paragraphs).
func blockScalar(key, value string) string {
	// An empty value (a freshly published draft may have blank fields) has no
	// sensible block-scalar form; emit a quoted empty string.
	if value == "" {
		return key + ": \"\"\n"
	}
	trailing := len(value) - len(strings.TrimRight(value, "\n"))
	content := value[:len(value)-trailing]

	indicator := ""
	keepExtra := 0
	switch {
	case trailing == 0:
		indicator = "-"
	case trailing == 1:
		indicator = "" // clip is the default; one trailing newline is implied
	default:
		indicator = "+"
		keepExtra = trailing - 1
	}

	var b strings.Builder
	b.WriteString(key + ": |" + indicator + "\n")
	for _, line := range strings.Split(content, "\n") {
		if line == "" {
			b.WriteByte('\n')
		} else {
			b.WriteString(refIndent + line + "\n")
		}
	}
	for i := 0; i < keepExtra; i++ {
		b.WriteByte('\n')
	}
	return b.String()
}

// plainScalar matches values safe to emit unquoted: they start with an
// alphanumeric, contain no YAML-significant run like ": " or " #", and aren't a
// reserved word. date ("2025/01/27"), author ("Outsider") and enclosure URLs
// all qualify, so they stay byte-identical to the source.
var plainScalar = regexp.MustCompile(`^[A-Za-z0-9][^\n]*$`)

// scalar emits a value as a plain YAML scalar when it is unambiguous, falling
// back to a double-quoted scalar otherwise.
func scalar(s string) string {
	if s == "" {
		return `""`
	}
	if plainScalar.MatchString(s) &&
		!strings.Contains(s, ": ") &&
		!strings.Contains(s, " #") &&
		!strings.HasSuffix(s, ":") &&
		!isReservedYAML(s) {
		return s
	}
	return doubleQuote(s)
}

// isReservedYAML reports whether a plain scalar would be parsed as something
// other than a string (bool/null), which must be quoted to stay a string.
func isReservedYAML(s string) bool {
	switch strings.ToLower(s) {
	case "true", "false", "yes", "no", "on", "off", "null", "~":
		return true
	}
	return false
}

// doubleQuote returns s as a double-quoted YAML scalar, escaping backslashes and
// quotes. Badge/enclosure URLs (which may contain "&" or "?") and duration are
// always safe inside double quotes.
func doubleQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

type badgeKV struct{ key, val string }

// nonEmptyBadges returns the set badge deep links in parser.Badges field order
// (apple, youtube, spotify, google, rss). Empty entries are dropped so the YAML
// omits them, matching the omitempty source convention.
func nonEmptyBadges(b parser.Badges) []badgeKV {
	all := []badgeKV{
		{"apple", b.Apple},
		{"youtube", b.YouTube},
		{"spotify", b.Spotify},
		{"google", b.Google},
		{"rss", b.RSS},
	}
	var out []badgeKV
	for _, kv := range all {
		if kv.val != "" {
			out = append(out, kv)
		}
	}
	return out
}
