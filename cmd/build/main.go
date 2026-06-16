// Command build renders the RetroTech static site into dist/: the home page,
// the episodes listing, one page per episode, the 404 page and the podcast RSS
// feed, then copies the static assets from public/. It replaces the old
// Next/Nextra build (and scripts/gen-rss.js).
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/outsideris/retrotech/internal/builder"
	"github.com/outsideris/retrotech/internal/parser"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "build failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	start := time.Now()

	base, err := os.Getwd()
	if err != nil {
		return err
	}
	contentDir := filepath.Join(base, "content", "episodes")
	publicDir := filepath.Join(base, "public")
	distDir := filepath.Join(base, "dist")

	site := builder.Site{
		Year: time.Now().Year(),
		// Only the deploy build sets ANALYTICS_ID, so local/CI/preview builds
		// ship no analytics — keeping them out of the production GA property.
		AnalyticsID: os.Getenv("ANALYTICS_ID"),
	}

	fmt.Println("Building site...")

	if err := os.RemoveAll(distDir); err != nil {
		return fmt.Errorf("clearing dist: %w", err)
	}
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return err
	}

	// Static assets first (images, badges, favicons, styles.css, _headers, …),
	// so the generated pages and feed are never clobbered by the copy.
	if err := copyDir(publicDir, distDir); err != nil {
		return fmt.Errorf("copying public: %w", err)
	}

	episodes, err := parser.LoadEpisodes(contentDir)
	if err != nil {
		return fmt.Errorf("loading episodes: %w", err)
	}
	if len(episodes) == 0 {
		return fmt.Errorf("no episodes found in %s", contentDir)
	}

	// Pages.
	if err := writeFile(filepath.Join(distDir, "index.html"), builder.BuildHomePage(episodes, site)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(distDir, "episodes.html"), builder.BuildEpisodesPage(episodes, site)); err != nil {
		return err
	}
	if err := writeFile(filepath.Join(distDir, "404.html"), builder.Build404Page(site)); err != nil {
		return err
	}
	for _, ep := range episodes {
		dst := filepath.Join(distDir, "episodes", ep.ID+".html")
		if err := writeFile(dst, builder.BuildEpisodePage(ep, site)); err != nil {
			return err
		}
	}

	// Podcast RSS feed.
	feed := builder.BuildFeed(episodes, builder.FeedConfig{SiteURL: "https://retrotech.outsider.dev"}, time.Now())
	if err := os.WriteFile(filepath.Join(distDir, "feed.xml"), feed, 0644); err != nil {
		return err
	}

	fmt.Printf("Built %d episodes + home/episodes/404/feed in %v\n", len(episodes), time.Since(start))
	return nil
}

// writeFile writes content to path, creating parent directories.
func writeFile(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

// copyDir recursively copies the contents of src into dst. Missing src is not
// an error (a project may have no public/ dir).
func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is not a directory", src)
	}

	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
