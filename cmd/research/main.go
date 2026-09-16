// Command research opens a local, anchored research reader for an HTML report.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/outsideris/retrotech/internal/research"
)

func main() {
	if e := run(); e != nil {
		log.Fatal(e)
	}
}
func run() error {
	report := flag.String("report", "", "Directory containing index.html and research.md")
	home, _ := os.UserHomeDir()
	skill := flag.String("skill", filepath.Join(home, ".codex/skills/research-retrotech"), "Research skill directory")
	portDefault := 49327
	if v := os.Getenv("PORT"); v != "" {
		n, e := strconv.Atoi(v)
		if e != nil {
			return e
		}
		portDefault = n
	}
	port := flag.Int("port", portDefault, "Preferred loopback port; falls back to a free port")
	openFlag := flag.Bool("open", false, "Open the report in the default browser")
	flag.Parse()
	if *report == "" {
		return fmt.Errorf("-report is required")
	}
	if *port < 0 || *port > 65535 || (*port >= 7800 && *port <= 7899) {
		return fmt.Errorf("invalid or terrarium-reserved port")
	}
	dir, e := filepath.Abs(*report)
	if e != nil {
		return e
	}
	dir, e = filepath.EvalSymlinks(dir)
	if e != nil {
		return e
	}
	for _, part := range strings.Split(filepath.ToSlash(dir), "/") {
		if part == "episodes" {
			return fmt.Errorf("episodes is immutable")
		}
	}
	meta := filepath.Join(dir, ".research")
	if e = os.MkdirAll(meta, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(meta, "server.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		var saved struct {
			Origin string `json:"origin"`
		}
		b, _ := os.ReadFile(filepath.Join(meta, "server.json"))
		_ = json.Unmarshal(b, &saved)
		if strings.HasPrefix(saved.Origin, "http://127.0.0.1:") && sameReport(saved.Origin, dir) {
			if *openFlag {
				openBrowser(saved.Origin)
			}
			fmt.Println(saved.Origin)
			return nil
		}
		return fmt.Errorf("another research server holds this report")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	ln, e := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if e != nil {
		ln, e = net.Listen("tcp", "127.0.0.1:0")
	}
	if e != nil {
		return e
	}
	defer ln.Close()
	origin := "http://" + ln.Addr().String()
	store, e := research.NewStore(dir, *skill, origin)
	if e != nil {
		return e
	}
	app := research.NewServer(store, nil)
	defer app.Close()
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	script := "#!/bin/zsh\n# RetroTech local research reader\nexec " + quote(exe) + " -report " + quote(dir) + " -skill " + quote(*skill) + " -open\n"
	launcher := filepath.Join(dir, "start-research.command")
	if old, e := os.ReadFile(launcher); e == nil && !strings.Contains(string(old), "# RetroTech local research reader") {
		return fmt.Errorf("existing launcher belongs to another workflow")
	}
	if e = os.WriteFile(launcher, []byte(script), 0755); e != nil {
		return e
	}
	b, _ := json.Marshal(map[string]string{"origin": origin})
	if e = os.WriteFile(filepath.Join(meta, "server.json"), b, 0600); e != nil {
		return e
	}
	srv := &http.Server{Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(done)
	go func() {
		<-done
		app.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()
	fmt.Printf("%s RESEARCH_URL %s\n", time.Now().Format(time.RFC3339), origin)
	if *openFlag {
		openBrowser(origin)
	}
	if e = srv.Serve(ln); e != http.ErrServerClosed {
		return e
	}
	return nil
}
func quote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'" }
func openBrowser(u string) {
	if runtime.GOOS == "darwin" {
		_ = exec.Command("open", u).Start()
	} else {
		_ = exec.Command("xdg-open", u).Start()
	}
}
func sameReport(origin, dir string) bool {
	client := http.Client{Timeout: time.Second}
	r, e := client.Get(origin + "/health")
	if e != nil {
		return false
	}
	defer r.Body.Close()
	var v struct {
		ID string `json:"reportId"`
	}
	if json.NewDecoder(r.Body).Decode(&v) != nil {
		return false
	}
	h := sha256.Sum256([]byte(dir))
	return v.ID == hex.EncodeToString(h[:])[:16]
}
