// Command serve previews the built site in dist/ over HTTP, resolving
// extensionless URLs (e.g. /episodes/2g → episodes/2g.html) the way Cloudflare
// Pages does, so local preview matches production. Run `go run ./cmd/build`
// first, then `go run ./cmd/serve` and open the URL it prints.
//
// The port is chosen automatically (the first free port from 8081 up, leaving
// 8080 for other projects); set $PORT to force a specific one.
package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	dir := "dist"

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
		// Serve the 404 page with a 404 status. Write it directly rather than
		// via ServeFile, which would also try to set the status (200) and log a
		// "superfluous WriteHeader" warning.
		body, err := os.ReadFile(filepath.Join(dir, "404.html"))
		if err != nil {
			http.Error(w, "404 page not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write(body)
	})

	ln, err := listen()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Serving %s/ at http://localhost:%d\n", dir, ln.Addr().(*net.TCPAddr).Port)
	if err := http.Serve(ln, nil); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// listen returns a TCP listener on a free local port. It honors $PORT when set;
// otherwise it takes the first free port from 8081 upward (leaving 8080 for
// other projects), falling back to any OS-assigned free port.
func listen() (net.Listener, error) {
	if p := os.Getenv("PORT"); p != "" {
		return net.Listen("tcp", "localhost:"+p)
	}
	for port := 8081; port <= 8130; port++ {
		if ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port)); err == nil {
			return ln, nil
		}
	}
	return net.Listen("tcp", "localhost:0")
}
