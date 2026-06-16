// Command serve previews the built site in dist/ over HTTP, resolving
// extensionless URLs (e.g. /episodes/2g → episodes/2g.html) the way Cloudflare
// Pages does, so local preview matches production. Run `go run ./cmd/build`
// first, then `go run ./cmd/serve` and open http://localhost:8080.
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := "dist"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		p := filepath.Clean(r.URL.Path)
		candidates := []string{filepath.Join(dir, p)}
		if !strings.HasSuffix(p, ".html") && p != "/" {
			candidates = append(candidates, filepath.Join(dir, p+".html"))
		}
		if p == "/" {
			candidates = []string{filepath.Join(dir, "index.html")}
		}
		for _, c := range candidates {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				http.ServeFile(w, r, c)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
		http.ServeFile(w, r, filepath.Join(dir, "404.html"))
	})

	addr := ":" + port
	fmt.Printf("Serving %s/ at http://localhost%s\n", dir, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
