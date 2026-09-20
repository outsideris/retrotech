// Package assist runs one-shot prompts through the local AI CLIs
// (Claude / Codex / Gemini), shelling out the same way the
// blog.outsider.ne.kr editor does. It is the foundation for editor AI
// features: it reports which CLIs are installed, runs a prompt through one of
// them with optional model/effort tuning, and returns per-call telemetry
// (duration, tokens, cost) for the sidebar's usage trace.
//
// Each provider is a thin wrapper over its CLI's non-interactive mode. The CLIs
// authenticate themselves (keychain / login), so this package owns no API keys.
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
	"time"
)

// ErrUnavailable means the provider's CLI is not installed. The HTTP layer maps
// it to 503 so the UI can say "install the CLI".
var ErrUnavailable = errors.New("provider CLI not installed")

// Options carry per-call tuning. Empty fields mean "use the CLI default" — the
// model the CLI is configured for, and no effort flag at all. A non-empty
// value must name one of the provider's catalog models (see catalog.go) and a
// level that model accepts; Validate checks both before anything runs.
type Options struct {
	Model  string
	Effort string
}

// Meta is the per-call telemetry the Assist trace shows. Fields the CLI doesn't
// report stay zero and the trace skips them.
type Meta struct {
	Provider     string  `json:"provider,omitempty"`
	Model        string  `json:"model,omitempty"`
	Effort       string  `json:"effort,omitempty"`
	DurationMs   int     `json:"durationMs,omitempty"`
	InputTokens  int     `json:"inputTokens,omitempty"`
	OutputTokens int     `json:"outputTokens,omitempty"`
	CostUSD      float64 `json:"costUsd,omitempty"`
}

// Provider runs a prompt through one local AI CLI.
type Provider interface {
	// Name is the stable key the UI selects on ("claude"/"codex"/"gemini").
	Name() string
	// Available reports whether the CLI is installed.
	Available() bool
	// Models lists the models the sidebar may offer for this CLI, newest
	// first; nil for a CLI with no model choice.
	Models() []Model
	// Run sends the prompt to the CLI and returns its text response + telemetry.
	Run(ctx context.Context, prompt string, opts Options) (string, Meta, error)
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
func (claude) Models() []Model   { return claudeModels }

func (c claude) Run(ctx context.Context, prompt string, opts Options) (string, Meta, error) {
	bin, ok := c.bin()
	if !ok {
		return "", Meta{}, fmt.Errorf("claude: %w", ErrUnavailable)
	}
	if err := validateOptions("claude", claudeModels, opts); err != nil {
		return "", Meta{}, err
	}
	args := []string{"-p", "--output-format=json"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Effort != "" {
		args = append(args, "--effort", opts.Effort)
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	elapsed := int(time.Since(start).Milliseconds())
	// claude exits non-zero on API errors (e.g. 401) but still prints its JSON
	// envelope, which carries the real message — prefer it over "exit status 1".
	if len(bytes.TrimSpace(out)) > 0 {
		text, meta, perr := parseClaude(out)
		meta.Model, meta.Effort = opts.Model, opts.Effort
		if meta.DurationMs == 0 {
			meta.DurationMs = elapsed
		}
		return text, meta, perr
	}
	if err != nil {
		return "", Meta{}, runErr("claude", err, stderr.Bytes())
	}
	return "", Meta{}, errors.New("claude: empty response")
}

// parseClaude pulls the response text + telemetry out of `claude -p
// --output-format=json`'s envelope, falling back to raw stdout if it isn't the
// expected JSON.
func parseClaude(stdout []byte) (string, Meta, error) {
	var env struct {
		Result       string  `json:"result"`
		IsError      bool    `json:"is_error"`
		DurationMs   int     `json:"duration_ms"`
		TotalCostUSD float64 `json:"total_cost_usd"`
		Usage        struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(bytes.TrimSpace(stdout), &env) == nil && (env.Result != "" || env.IsError) {
		if env.IsError {
			return "", Meta{}, fmt.Errorf("claude: %s", strings.TrimSpace(env.Result))
		}
		meta := Meta{
			DurationMs:   env.DurationMs,
			InputTokens:  env.Usage.InputTokens,
			OutputTokens: env.Usage.OutputTokens,
			CostUSD:      env.TotalCostUSD,
		}
		return strings.TrimSpace(env.Result), meta, nil
	}
	if s := strings.TrimSpace(string(stdout)); s != "" {
		return s, Meta{}, nil
	}
	return "", Meta{}, errors.New("claude: empty response")
}

// ---------- Codex ----------

type codex struct{}

func (codex) Name() string { return "codex" }
func (codex) bin() (string, bool) {
	return resolveBinary([]string{"codex"}, homePaths(".local/bin/codex", "bin/codex"))
}
func (c codex) Available() bool { _, ok := c.bin(); return ok }
func (codex) Models() []Model   { return codexModels }

func (c codex) Run(ctx context.Context, prompt string, opts Options) (string, Meta, error) {
	bin, ok := c.bin()
	if !ok {
		return "", Meta{}, fmt.Errorf("codex: %w", ErrUnavailable)
	}
	if err := validateOptions("codex", codexModels, opts); err != nil {
		return "", Meta{}, err
	}
	// --sandbox read-only: the agent can't write to disk;
	// --skip-git-repo-check: runs anywhere; --json: machine-readable stream.
	args := []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "read-only"}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Effort != "" {
		// Codex exposes effort only as a config override, not a flag.
		args = append(args, "-c", fmt.Sprintf(`model_reasoning_effort="%s"`, opts.Effort))
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Stdin = strings.NewReader(prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	elapsed := int(time.Since(start).Milliseconds())
	// Codex reports errors as JSONL "error" events on stdout, so parse the
	// stream when there is one; only a truly empty stream is a process failure.
	if len(bytes.TrimSpace(out)) > 0 {
		text, meta, perr := parseCodex(out)
		meta.Model, meta.Effort, meta.DurationMs = opts.Model, opts.Effort, elapsed
		return text, meta, perr
	}
	if err != nil {
		return "", Meta{}, runErr("codex", err, stderr.Bytes())
	}
	return "", Meta{}, errors.New("codex: empty response")
}

// parseCodex consumes codex's JSONL stream and returns the concatenated
// agent-message text + token usage. An "error" event becomes the error. Codex
// doesn't echo a duration or cost in the stream, so the caller fills the
// wall-clock duration.
func parseCodex(stdout []byte) (string, Meta, error) {
	sc := bufio.NewScanner(bytes.NewReader(stdout))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024) // long agent messages
	var msg strings.Builder
	var meta Meta
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
			Usage *struct {
				InputTokens  int `json:"input_tokens"`
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
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
			return "", Meta{}, fmt.Errorf("codex: %s", detail)
		case "item.completed":
			if ev.Item != nil && ev.Item.Type == "agent_message" {
				msg.WriteString(ev.Item.Text)
			}
		case "turn.completed":
			if ev.Usage != nil {
				meta.InputTokens, meta.OutputTokens = ev.Usage.InputTokens, ev.Usage.OutputTokens
			}
		}
	}
	if err := sc.Err(); err != nil {
		return "", Meta{}, fmt.Errorf("codex: scan stdout: %w", err)
	}
	if out := strings.TrimSpace(msg.String()); out != "" {
		return out, meta, nil
	}
	return "", Meta{}, errors.New("codex: empty response")
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
func (gemini) Models() []Model   { return nil }

func (g gemini) Run(ctx context.Context, prompt string, opts Options) (string, Meta, error) {
	bin, ok := g.bin()
	if !ok {
		return "", Meta{}, fmt.Errorf("gemini: %w", ErrUnavailable)
	}
	// The agy/gemini CLI exposes no model/effort flags. Its catalog is empty,
	// so validateOptions refuses any model or effort rather than accepting
	// tuning this CLI would quietly drop.
	if err := validateOptions("gemini", nil, opts); err != nil {
		return "", Meta{}, err
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, bin, "-p", prompt)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	elapsed := int(time.Since(start).Milliseconds())
	if s := strings.TrimSpace(string(out)); s != "" {
		return s, Meta{DurationMs: elapsed}, nil
	}
	if err != nil {
		return "", Meta{}, runErr("gemini", err, stderr.Bytes())
	}
	return "", Meta{}, errors.New("gemini: empty response")
}
