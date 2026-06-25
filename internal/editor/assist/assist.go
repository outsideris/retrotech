// Package assist runs one-shot prompts through the local AI CLIs
// (Claude / Codex / Gemini), shelling out the same way the
// blog.outsider.ne.kr editor does. It is the foundation for editor AI
// features: for now it only reports which CLIs are installed and runs a
// prompt through one of them; specific features are layered on later.
//
// Each provider is a thin wrapper over its CLI's non-interactive mode.
// The CLIs authenticate themselves (keychain / login), so this package
// owns no API keys.
package assist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrUnavailable means the provider's CLI is not on PATH. The HTTP layer
// maps it to 503 so the UI can say "install the CLI".
var ErrUnavailable = errors.New("provider CLI not installed")

// Provider runs a prompt through one local AI CLI.
type Provider interface {
	// Name is the stable key the UI selects on ("claude"/"codex"/"gemini").
	Name() string
	// Available reports whether the CLI is on PATH.
	Available() bool
	// Run sends the prompt to the CLI and returns its text response.
	Run(ctx context.Context, prompt string) (string, error)
}

// Providers returns the supported providers in display order.
func Providers() []Provider {
	return []Provider{claude{}, codex{}, gemini{}}
}

// Find returns the provider registered under name.
func Find(name string) (Provider, bool) {
	for _, p := range Providers() {
		if p.Name() == name {
			return p, true
		}
	}
	return nil, false
}

// resolveBinary finds a provider's CLI: the first of names found on PATH, else
// the first existing fallback path. A GUI app's PATH may miss the dir a CLI
// lives in (or the CLI may be named differently — e.g. the Gemini provider runs
// the `agy` wrapper), so each provider supplies both name aliases and the known
// install locations. ok is false when nothing resolves.
func resolveBinary(names, fallbacks []string) (string, bool) {
	for _, n := range names {
		if p, err := exec.LookPath(n); err == nil {
			return p, true
		}
	}
	for _, f := range fallbacks {
		if info, err := os.Stat(f); err == nil && !info.IsDir() {
			return f, true
		}
	}
	return "", false
}

// homePaths joins each rel under the user's home dir (for fallback locations
// like ~/.local/bin/<cli>). Returns nil if the home dir is unknown.
func homePaths(rel ...string) []string {
	h, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	out := make([]string, len(rel))
	for i, r := range rel {
		out[i] = filepath.Join(h, r)
	}
	return out
}

// runErr turns a failed CLI run into a useful error, preferring the CLI's
// stderr over Go's generic "exit status N".
func runErr(name string, err error, stderr []byte) error {
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("%s: %w", name, ErrUnavailable)
	}
	if msg := strings.TrimSpace(string(stderr)); msg != "" {
		return fmt.Errorf("%s: %s", name, msg)
	}
	return fmt.Errorf("%s: %v", name, err)
}

// ---------- Claude ----------

type claude struct{}

func (claude) Name() string { return "claude" }
func (claude) bin() (string, bool) {
	return resolveBinary([]string{"claude"}, homePaths(".local/bin/claude"))
}
func (c claude) Available() bool { _, ok := c.bin(); return ok }

func (c claude) Run(ctx context.Context, prompt string) (string, error) {
	bin, ok := c.bin()
	if !ok {
		return "", fmt.Errorf("claude: %w", ErrUnavailable)
	}
	cmd := exec.CommandContext(ctx, bin, "-p", "--output-format=json")
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	// claude exits non-zero on API errors (e.g. 401) but still prints its JSON
	// envelope, which carries the real message — prefer it over "exit status 1".
	if len(bytes.TrimSpace(out)) > 0 {
		return parseClaude(out)
	}
	if err != nil {
		return "", runErr("claude", err, stderr.Bytes())
	}
	return "", errors.New("claude: empty response")
}

// parseClaude pulls the response text out of `claude -p --output-format=json`'s
// envelope, falling back to the raw stdout if it isn't the expected JSON.
func parseClaude(stdout []byte) (string, error) {
	var env struct {
		Result  string `json:"result"`
		IsError bool   `json:"is_error"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &env) == nil && (env.Result != "" || env.IsError) {
		if env.IsError {
			return "", fmt.Errorf("claude: %s", strings.TrimSpace(env.Result))
		}
		return strings.TrimSpace(env.Result), nil
	}
	if s := strings.TrimSpace(string(stdout)); s != "" {
		return s, nil
	}
	return "", errors.New("claude: empty response")
}

// ---------- Codex ----------

type codex struct{}

func (codex) Name() string { return "codex" }
func (codex) bin() (string, bool) {
	return resolveBinary([]string{"codex"}, homePaths(".local/bin/codex", "bin/codex"))
}
func (c codex) Available() bool { _, ok := c.bin(); return ok }

func (c codex) Run(ctx context.Context, prompt string) (string, error) {
	bin, ok := c.bin()
	if !ok {
		return "", fmt.Errorf("codex: %w", ErrUnavailable)
	}
	// --sandbox read-only: the agent can't write to disk;
	// --skip-git-repo-check: runs anywhere; --json: machine-readable stream.
	cmd := exec.CommandContext(ctx, bin, "exec", "--json", "--skip-git-repo-check", "--sandbox", "read-only")
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	// Codex reports errors as JSONL "error" events on stdout, so parse the
	// stream when there is one; only a truly empty stream is a process failure.
	if len(bytes.TrimSpace(out)) > 0 {
		return parseCodex(out)
	}
	if err != nil {
		return "", runErr("codex", err, stderr.Bytes())
	}
	return "", errors.New("codex: empty response")
}

// parseCodex consumes codex's JSONL stream and returns the concatenated
// agent-message text. An "error" event becomes the error.
func parseCodex(stdout []byte) (string, error) {
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // long agent messages
	var msg strings.Builder
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev struct {
			Type    string `json:"type"`
			Message string `json:"message"`
			Item    *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if json.Unmarshal(line, &ev) != nil {
			continue // tolerate the occasional non-JSON warning line
		}
		switch ev.Type {
		case "error":
			detail := strings.TrimSpace(ev.Message)
			if detail == "" {
				detail = "codex reported an error"
			}
			return "", fmt.Errorf("codex: %s", detail)
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" {
				msg.WriteString(ev.Item.Text)
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", fmt.Errorf("codex: scan stdout: %w", err)
	}
	if out := strings.TrimSpace(msg.String()); out != "" {
		return out, nil
	}
	return "", errors.New("codex: empty response")
}

// ---------- Gemini ----------

type gemini struct{}

func (gemini) Name() string { return "gemini" }
func (gemini) bin() (string, bool) {
	// The blog's Gemini provider runs `agy` (the antigravity CLI); accept the
	// `gemini` name too, with the standard ~/.local/bin installs as fallback.
	return resolveBinary([]string{"gemini", "agy"}, homePaths(".local/bin/agy", ".local/bin/gemini"))
}
func (g gemini) Available() bool { _, ok := g.bin(); return ok }

func (g gemini) Run(ctx context.Context, prompt string) (string, error) {
	bin, ok := g.bin()
	if !ok {
		return "", fmt.Errorf("gemini: %w", ErrUnavailable)
	}
	// The Gemini CLI (gemini/agy) prints a plain-text answer in -p mode.
	cmd := exec.CommandContext(ctx, bin, "-p", prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if s := strings.TrimSpace(string(out)); s != "" {
		return s, nil
	}
	if err != nil {
		return "", runErr("gemini", err, stderr.Bytes())
	}
	return "", errors.New("gemini: empty response")
}
