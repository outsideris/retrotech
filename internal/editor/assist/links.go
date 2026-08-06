package assist

import (
	"regexp"
	"sort"
	"strings"
)

// Link is one external link found in a podcast script: the URL plus the
// script's anchor text (empty for a bare URL). The anchor text is only a hint —
// reference titles come from the linked page, not from how the script happened
// to phrase the link (see analyzePrompt's naming rules).
type Link struct {
	Text string
	URL  string
}

// Link extraction is done in Go, not by the model, so that no link in the
// script can be silently dropped: the model only titles the URLs it is given
// and reconcileRefs restores any it loses.
var (
	// mdLinkRe matches a markdown link with an http(s) target, tolerating one
	// level of parentheses in the URL (Wikipedia-style).
	mdLinkRe = regexp.MustCompile(`\[([^\]]*)\]\((https?://(?:[^()\s]|\([^()\s]*\))*)\)`)
	// bareURLRe matches an http(s) URL in plain text, likewise tolerating one
	// level of balanced parentheses.
	bareURLRe = regexp.MustCompile(`https?://(?:[^\s<>()]|\([^\s()]*\))+`)
)

// ExtractLinks returns every external link in a markdown script, in document
// order, deduplicated by URL (the first occurrence wins). Markdown links carry
// their anchor text; bare URLs have none. Images (![...](...)) are skipped —
// a reference list is a list of links, not embeds.
func ExtractLinks(script string) []Link {
	type candidate struct {
		pos  int
		text string
		url  string
	}
	var found []candidate

	// Markdown links first, remembering their spans so the bare-URL pass
	// doesn't re-extract the URL inside "[text](url)".
	type span struct{ start, end int }
	var covered []span
	for _, m := range mdLinkRe.FindAllStringSubmatchIndex(script, -1) {
		covered = append(covered, span{m[0], m[1]})
		if m[0] > 0 && script[m[0]-1] == '!' {
			continue // image, not a reference
		}
		found = append(found, candidate{m[0], script[m[2]:m[3]], script[m[4]:m[5]]})
	}
	for _, m := range bareURLRe.FindAllStringIndex(script, -1) {
		inside := false
		for _, s := range covered {
			if m[0] >= s.start && m[1] <= s.end {
				inside = true
				break
			}
		}
		if !inside {
			found = append(found, candidate{m[0], "", script[m[0]:m[1]]})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].pos < found[j].pos })

	var links []Link
	seen := map[string]bool{}
	for _, c := range found {
		url := strings.TrimRight(c.url, `.,;:!?'"`)
		if url == "" || seen[url] {
			continue
		}
		seen[url] = true
		links = append(links, Link{Text: strings.TrimSpace(c.text), URL: url})
	}
	return links
}
