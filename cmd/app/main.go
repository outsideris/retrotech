// Command app is the HTTP backend for the RetroTech episode editor desktop app.
//
// It serves the episode management UI and its JSON API on a loopback port and
// prints the chosen port to stdout ("EDITOR_PORT <n>") so the Electron main
// process (desktop/) can load it in a BrowserWindow. Electron owns the window,
// the folder picker, the settings and the lifecycle — this binary is only the
// server. It is pure Go, so it builds everywhere and needs no toolchain at
// runtime once compiled.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/outsideris/retrotech/internal/editor"
)

// editorPort is the fixed loopback port the editor prefers. A stable port keeps
// the BrowserWindow's web origin constant across restarts so the page's
// localStorage (e.g. UI preferences) survives. It sits in the IANA dynamic
// range and is distinct from cmd/serve's 8081; listenLoopback falls back to an
// OS-assigned port if it is busy so the app still launches.
const editorPort = 49218

func main() {
	repoFlag := flag.String("repo", "", "RetroTech project root (must contain content/episodes). Required.")
	flag.Parse()

	repoDir := strings.TrimSpace(*repoFlag)
	if repoDir == "" {
		fatalf("missing required -repo flag")
	}

	ed, err := editor.New(editor.Config{RepoDir: repoDir})
	if err != nil {
		fatalf("%v", err)
	}

	ln, err := listenLoopback(editorPort)
	if err != nil {
		fatalf("cannot open a local port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	srv := &http.Server{Handler: ed.Handler()}

	// Hand the port to the Electron parent on a single stdout line; it reads
	// this to know which URL to load.
	fmt.Printf("EDITOR_PORT %d\n", port)

	// Shut down gracefully when Electron terminates us on quit.
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatalf("serve: %v", err)
	}
}

// listenLoopback binds 127.0.0.1:port, falling back to an OS-assigned loopback
// port when that one is already in use so the app still starts. The caller reads
// the actual bound port back from the returned listener.
func listenLoopback(port int) (net.Listener, error) {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return net.Listen("tcp", "127.0.0.1:0")
	}
	return ln, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
