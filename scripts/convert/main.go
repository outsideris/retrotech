// Command convert is the one-time migration tool that turns the old
// pages/episodes/*.mdx show notes into content/episodes/*.md for the Go
// builder. It preserves the frontmatter verbatim (so the RSS feed stays
// byte-identical) and only appends a `badges:` map extracted from the body's
// <Badges/> element; the body is rewritten to drop the title h1 and the Badges
// import/JSX (replaced with a <!--badges--> marker) and to make the references
// block valid CommonMark.
//
// Usage: go run ./scripts/convert
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

var (
	badgesRE = regexp.MustCompile(`(?s)<Badges(.*?)/>`)
	attrRE   = regexp.MustCompile(`(\w+)\s*=\s*"([^"]*)"`)
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	src := filepath.Join("pages", "episodes")
	dst := filepath.Join("content", "episodes")
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	count := 0
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if e.IsDir() || (ext != ".mdx" && ext != ".md") || strings.HasPrefix(name, "index.") {
			continue
		}
		id := strings.TrimSuffix(name, ext)
		out, err := convert(filepath.Join(src, name))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(dst, id+".md"), []byte(out), 0644); err != nil {
			return err
		}
		count++
	}
	fmt.Printf("converted %d episodes\n", count)
	return nil
}

func convert(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	fmBytes, bodyBytes, err := parser.SplitFrontmatterAndBody(data)
	if err != nil {
		return "", err
	}
	frontmatter := string(fmBytes)
	body := string(bodyBytes)

	// Extract the per-episode subscription deep links from <Badges .../>.
	badgesYAML := ""
	if m := badgesRE.FindStringSubmatch(body); m != nil {
		badgesYAML = badgesToYAML(m[1])
	}

	newBody := transformBody(body)

	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString(frontmatter)
	b.WriteString("\n")
	b.WriteString(badgesYAML)
	b.WriteString("---\n\n")
	b.WriteString(newBody)
	b.WriteString("\n")
	return b.String(), nil
}

// badgesToYAML turns the attributes captured from <Badges ...> into a YAML
// `badges:` block. Only known platform keys are kept.
func badgesToYAML(attrs string) string {
	keep := map[string]bool{"apple": true, "youtube": true, "spotify": true, "google": true, "rss": true}
	var b strings.Builder
	for _, m := range attrRE.FindAllStringSubmatch(attrs, -1) {
		key, val := m[1], m[2]
		if !keep[key] {
			continue
		}
		b.WriteString("  " + key + ": \"" + val + "\"\n")
	}
	if b.Len() == 0 {
		return ""
	}
	return "badges:\n" + b.String()
}

// transformBody rewrites an episode body for the Go builder.
func transformBody(body string) string {
	lines := strings.Split(body, "\n")
	var out []string
	titleStripped := false
	inBadges := false
	inRefs := false
	badgesEmitted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		switch {
		case !titleStripped && strings.HasPrefix(line, "# "):
			titleStripped = true // drop the title h1 (the template emits it)
		case trimmed == "import Badges from 'components/Badges'":
			// drop the import
		case strings.HasPrefix(trimmed, "<Badges"):
			inBadges = true
			if strings.Contains(line, "/>") {
				inBadges = false
				if !badgesEmitted {
					out = append(out, "<!--badges-->")
					badgesEmitted = true
				}
			}
		case inBadges:
			if strings.Contains(line, "/>") {
				inBadges = false
				if !badgesEmitted {
					out = append(out, "<!--badges-->")
					badgesEmitted = true
				}
			}
		case strings.Contains(trimmed, `className="refs"`):
			out = append(out, `<div class="refs">`, "")
			inRefs = true
		case inRefs && trimmed == "</div>":
			out = append(out, "", "</div>")
			inRefs = false
		case inRefs:
			out = append(out, dedent(line))
		default:
			out = append(out, line)
		}
	}

	return collapseBlankRuns(strings.TrimSpace(strings.Join(out, "\n")))
}

// dedent removes up to four leading spaces so reference list items are
// top-level markdown (4-space indents would otherwise become a code block).
func dedent(line string) string {
	for i := 0; i < 4 && strings.HasPrefix(line, " "); i++ {
		line = line[1:]
	}
	return line
}

// collapseBlankRuns reduces any run of blank lines to a single blank line.
func collapseBlankRuns(s string) string {
	re := regexp.MustCompile(`\n{3,}`)
	return re.ReplaceAllString(s, "\n\n")
}
