package builder

// Podcasting 2.0 chapters (https://podcastindex.org/namespace/1.0): each
// episode with a `chapters:` frontmatter list gets a JSON file next to its
// page ("episodes/2g.chapters.json"), and its feed item links it via
// <podcast:chapters>. Apps that support the namespace (Overcast, Pocket
// Casts, …) show the chapter list and seek on tap; apps that don't still get
// the plain-text timestamps appended to the feed description (see feed.go).

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

// chaptersJSONVersion is the Podcasting 2.0 chapters format version.
const chaptersJSONVersion = "1.2.0"

// ChaptersRelPath is the dist-relative (and URL) path of an episode's
// chapters JSON. It sits beside the episode page as a plain file so the
// static host serves it with no extra routing.
func ChaptersRelPath(id string) string {
	return "episodes/" + id + ".chapters.json"
}

type chaptersDoc struct {
	Version  string        `json:"version"`
	Chapters []chapterJSON `json:"chapters"`
}

type chapterJSON struct {
	StartTime int    `json:"startTime"`
	Title     string `json:"title"`
}

// BuildChaptersJSON renders the Podcasting 2.0 chapters JSON for an episode,
// or nil when the episode declares no chapters. Chapters are emitted in
// frontmatter order (the spec expects playback order).
func BuildChaptersJSON(ep parser.Episode) ([]byte, error) {
	if len(ep.Chapters) == 0 {
		return nil, nil
	}

	doc := chaptersDoc{Version: chaptersJSONVersion}
	for i, ch := range ep.Chapters {
		secs, err := ch.StartSeconds()
		if err != nil {
			return nil, fmt.Errorf("episode %s: chapter %d: %w", ep.ID, i+1, err)
		}
		doc.Chapters = append(doc.Chapters, chapterJSON{
			StartTime: secs,
			Title:     strings.TrimSpace(ch.Title),
		})
	}

	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
