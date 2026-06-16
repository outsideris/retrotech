// Package parser loads RetroTech episodes from markdown files: it splits YAML
// frontmatter from the body, unmarshals the metadata, and renders the body to
// HTML with goldmark. It is the single source the page builder and the RSS
// feed both read from — mirroring the old MDX-frontmatter / scripts/gen-rss.js
// split, but without Next or Nextra.
package parser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	frontmatterDelimiter = []byte("---")
	errNoFrontmatter     = errors.New("frontmatter not found")
)

// dateLayout is the source date form used in episode frontmatter. Months and
// days are not always zero-padded ("2024/9/18" as well as "2025/01/27"), so the
// non-padded reference layout is used — it accepts both one- and two-digit
// fields.
const dateLayout = "2006/1/2"

// Enclosure is the podcast audio attachment referenced by an episode.
type Enclosure struct {
	URL  string `yaml:"url"`
	Size int64  `yaml:"size"`
}

// Badges holds the per-episode subscription deep links. Empty fields fall back
// to the show-level defaults in the badges renderer (the same defaults the old
// components/Badges.tsx baked in); when Google is empty the renderer shows the
// RSS badge instead — identical rule to the React component.
type Badges struct {
	Apple   string `yaml:"apple,omitempty"`
	YouTube string `yaml:"youtube,omitempty"`
	Spotify string `yaml:"spotify,omitempty"`
	Google  string `yaml:"google,omitempty"`
	RSS     string `yaml:"rss,omitempty"`
}

// Frontmatter mirrors the YAML frontmatter of an episode.
//
// Date stays a string in the source "YYYY/MM/DD" form so the feed reproduces
// the exact pubDate the old scripts/gen-rss.js emitted (see internal/builder
// feed generation); ParsedDate exposes it as time.Time for ordering. Title,
// Description and Description2 are kept verbatim (including any trailing
// newline produced by a folded/literal YAML block scalar) because the feed
// embeds them unchanged — trimming here would diverge from the current feed.
type Frontmatter struct {
	Title        string    `yaml:"title"`
	Date         string    `yaml:"date"`
	Description  string    `yaml:"description"`
	Description2 string    `yaml:"description2,omitempty"`
	Author       string    `yaml:"author"`
	Enclosure    Enclosure `yaml:"enclosure"`
	Duration     string    `yaml:"duration"`
	Badges       Badges    `yaml:"badges,omitempty"`
}

// ParsedDate parses the source "YYYY/MM/DD" date for ordering. An unparseable
// date yields the zero time (sorts last).
func (f Frontmatter) ParsedDate() time.Time {
	t, err := time.Parse(dateLayout, strings.TrimSpace(f.Date))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Episode is a parsed episode: its frontmatter, the id derived from the
// filename (the URL slug — "2g", "0", "250127-breaks"), and the raw markdown
// body. Rendering the body to HTML is the builder's job (it needs GFM and the
// Nextra-equivalent transforms), so the parser keeps the source verbatim.
type Episode struct {
	Frontmatter
	ID   string
	Body string
}

// ParseFrontmatter unmarshals YAML frontmatter bytes into a Frontmatter.
func ParseFrontmatter(data []byte) (Frontmatter, error) {
	var fm Frontmatter
	if err := yaml.Unmarshal(data, &fm); err != nil {
		return Frontmatter{}, err
	}
	return fm, nil
}

// LoadEpisode reads one markdown file and parses it into an Episode. The id is
// the filename without its extension.
func LoadEpisode(path string) (Episode, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Episode{}, err
	}

	fmBytes, bodyBytes, err := SplitFrontmatterAndBody(data)
	if err != nil {
		return Episode{}, fmt.Errorf("%s: %w", path, err)
	}

	fm, err := ParseFrontmatter(fmBytes)
	if err != nil {
		return Episode{}, fmt.Errorf("parsing frontmatter in %s: %w", path, err)
	}

	base := filepath.Base(path)
	id := strings.TrimSuffix(base, filepath.Ext(base))

	return Episode{
		Frontmatter: fm,
		ID:          id,
		Body:        string(bodyBytes),
	}, nil
}

// LoadEpisodes loads every `*.md` file under dir (skipping `index.*` listing
// pages), and returns them ordered newest-first. Same-date episodes fall back
// to id descending so the order is stable across builds and filesystems —
// matching scripts/gen-rss.js's sortByDateDesc.
func LoadEpisodes(dir string) ([]Episode, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var episodes []Episode
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		if strings.HasPrefix(entry.Name(), "index.") {
			continue
		}
		ep, err := LoadEpisode(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		episodes = append(episodes, ep)
	}

	SortEpisodes(episodes)
	return episodes, nil
}

// SortEpisodes orders episodes in place newest-first. Same-date episodes fall
// back to id descending so the order is stable across builds and filesystems —
// matching scripts/gen-rss.js's sortByDateDesc.
func SortEpisodes(episodes []Episode) {
	sort.SliceStable(episodes, func(i, j int) bool {
		di, dj := episodes[i].ParsedDate(), episodes[j].ParsedDate()
		if !di.Equal(dj) {
			return di.After(dj)
		}
		return episodes[i].ID > episodes[j].ID
	})
}

// SplitFrontmatterAndBody splits a markdown file's content into frontmatter and
// body. The file must start with a "---" line and have a closing "---" on its
// own line. The closing delimiter must match a line that is exactly "---"
// (optionally with trailing whitespace) so long dash runs inside a YAML block
// scalar don't misfire.
func SplitFrontmatterAndBody(data []byte) (frontmatter []byte, body []byte, err error) {
	data = bytes.TrimSpace(data)

	if !bytes.HasPrefix(data, frontmatterDelimiter) {
		return nil, nil, errNoFrontmatter
	}

	rest := data[len(frontmatterDelimiter):]
	if len(rest) > 0 && rest[0] == '\n' {
		rest = rest[1:]
	}

	closeStart, closeEnd := findFrontmatterClose(rest)
	if closeStart == -1 {
		return nil, nil, errNoFrontmatter
	}

	frontmatter = bytes.TrimSpace(rest[:closeStart])
	body = bytes.TrimSpace(rest[closeEnd:])

	return frontmatter, body, nil
}

// findFrontmatterClose returns the byte offsets of the closing "---" line in
// data: closeStart at the line's first byte, closeEnd past its trailing newline
// (or len(data) at EOF). Returns -1, -1 when no such line exists.
func findFrontmatterClose(data []byte) (closeStart, closeEnd int) {
	i := 0
	for i < len(data) {
		lineStart := i
		for i < len(data) && data[i] != '\n' {
			i++
		}
		line := data[lineStart:i]
		trimmed := bytes.TrimRight(line, " \t\r")
		if bytes.Equal(trimmed, frontmatterDelimiter) {
			if i < len(data) {
				return lineStart, i + 1
			}
			return lineStart, i
		}
		if i < len(data) {
			i++
		}
	}
	return -1, -1
}
