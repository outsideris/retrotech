package builder

import (
	"encoding/xml"
	"strings"

	"github.com/outsideris/retrotech/internal/parser"
)

const sitemapDateFmt = "2006-01-02"

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

// BuildSitemap renders sitemap.xml: the home page, the episodes listing, and
// one entry per episode. Each episode's lastmod is its publish date; the two
// landing pages take the newest episode's date, so adding an episode updates
// the whole sitemap on the next build. The 404 page and the RSS feed are left
// out on purpose (a 404 should not be indexed, and the feed is not a page).
func BuildSitemap(episodes []parser.Episode, siteURL string) ([]byte, error) {
	site := strings.TrimRight(siteURL, "/")

	// Newest episode date — used as lastmod for the landings that list episodes.
	// Computed by scanning so it does not depend on the input being pre-sorted.
	var latest string
	for _, ep := range episodes {
		if d := ep.ParsedDate().Format(sitemapDateFmt); d > latest {
			latest = d
		}
	}

	urls := []sitemapURL{
		{Loc: site + "/", LastMod: latest},
		{Loc: site + "/episodes", LastMod: latest},
	}
	for _, ep := range episodes {
		urls = append(urls, sitemapURL{
			Loc:     site + "/episodes/" + ep.ID,
			LastMod: ep.ParsedDate().Format(sitemapDateFmt),
		})
	}

	body, err := xml.MarshalIndent(sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), append(body, '\n')...), nil
}
