package builder

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/outsideris/retrotech/internal/parser"
)

func TestBuildSitemap(t *testing.T) {
	eps := []parser.Episode{
		{Frontmatter: parser.Frontmatter{Date: "2026/03/07"}, ID: "2g"},
		{Frontmatter: parser.Frontmatter{Date: "2025/12/26"}, ID: "2f"},
		{Frontmatter: parser.Frontmatter{Date: "2023/7/24"}, ID: "0"},
	}
	out, err := BuildSitemap(eps, "https://retrotech.outsider.dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	s := string(out)

	for _, want := range []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">`,
		"<loc>https://retrotech.outsider.dev/</loc>",
		"<loc>https://retrotech.outsider.dev/episodes</loc>",
		"<loc>https://retrotech.outsider.dev/episodes/2g</loc>",
		"<loc>https://retrotech.outsider.dev/episodes/0</loc>",
		"<lastmod>2023-07-24</lastmod>", // non-zero-padded source date renders padded
	} {
		if !strings.Contains(s, want) {
			t.Errorf("sitemap missing %q", want)
		}
	}

	// The 404 page must not be listed.
	if strings.Contains(s, "/404") {
		t.Error("sitemap should not include the 404 page")
	}

	// Valid XML, and the landing pages carry the newest episode's date.
	var set sitemapURLSet
	if err := xml.Unmarshal(out, &set); err != nil {
		t.Fatalf("sitemap is not valid XML: %v", err)
	}
	if len(set.URLs) != 5 { // home + episodes + 3 episodes
		t.Fatalf("expected 5 urls, got %d", len(set.URLs))
	}
	if set.URLs[0].LastMod != "2026-03-07" || set.URLs[1].LastMod != "2026-03-07" {
		t.Errorf("landing lastmod = %q/%q, want newest episode date 2026-03-07",
			set.URLs[0].LastMod, set.URLs[1].LastMod)
	}
}
