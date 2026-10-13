package assist

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// ResearchOptions confines a one-shot web investigation to structured output.
// The caller, not the model, owns all report and skill writes.
type ResearchOptions struct {
	Provider, Model, Effort, Directory, Schema string
	Progress                                   func(string)
}

// researchModels and researchEfforts are the research reader's own short list,
// kept apart from the Assist catalog on purpose: picks for source-checked web
// research, not everything a CLI can run. The reader's model menu
// (internal/research/assets/panel.html) offers exactly this list, and
// docs/RESEARCH_WORKBENCH.md says which pick suits which question.
//
// Refreshed with the CLIs on 2026-09-30 (claude 2.1.284, codex 0.159.0); every
// pick must stay runnable in the Assist catalog. Left out: GPT-6 Luna, the
// efficient tier for summarization and extraction; Fable 5.1, which can bill
// usage credits, and a -p run never asks before it does; Haiku 5.5, the fastest
// tier for quick answers; and the previous generation. Effort stops at xhigh:
// low is too shallow for checking sources, Claude documents max as prone to
// overthinking, and codex's ultra delegates to sub-agents, which the codex run
// below disables.
var (
	researchModels = map[string][]string{
		"codex":  {"gpt-6-sol", "gpt-6-astra"},
		"claude": {"claude-sonnet-5-5", "claude-opus-5-5"},
	}
	researchEfforts = []string{"medium", "high", "xhigh"}
)

// Why ValidateResearch turned a request down, kept apart so the reader can
// tell the user which choice to change.
var (
	ErrResearchProvider = errors.New("unsupported research provider")
	ErrResearchModel    = errors.New("unsupported research model")
	ErrResearchEffort   = errors.New("unsupported research effort")
)

// ValidateResearch reports whether the reader offers this provider, model and
// effort. researchArgs checks the same, so nothing else reaches a CLI.
func ValidateResearch(provider, model, effort string) error {
	if !slices.Contains(researchEfforts, effort) {
		return ErrResearchEffort
	}
	models, ok := researchModels[provider]
	if !ok {
		return ErrResearchProvider
	}
	if !slices.Contains(models, model) {
		return ErrResearchModel
	}
	return nil
}

func researchArgs(o ResearchOptions, schemaPath, resultPath string) ([]string, error) {
	if e := ValidateResearch(o.Provider, o.Model, o.Effort); e != nil {
		return nil, e
	}
	switch o.Provider {
	case "codex":
		return []string{"exec", "--ignore-user-config", "--ephemeral", "--skip-git-repo-check", "--sandbox", "read-only", "--json", "--color", "never", "--model", o.Model, "-c", `approval_policy="never"`, "-c", `web_search="live"`, "-c", `features.shell_tool=false`, "-c", `features.apps=false`, "-c", `features.multi_agent=false`, "-c", `model_reasoning_effort="` + o.Effort + `"`, "--output-schema", schemaPath, "--output-last-message", resultPath, "-"}, nil
	case "claude":
		// Safe mode preserves subscription authentication, unlike --bare. Only the
		// two web tools and the structured-output tool are available to this run.
		return []string{"--safe-mode", "-p", "--output-format", "stream-json", "--verbose", "--no-session-persistence", "--strict-mcp-config", "--mcp-config", `{"mcpServers":{}}`, "--tools", "WebSearch,WebFetch", "--allowedTools", "WebSearch,WebFetch", "--permission-mode", "dontAsk", "--model", o.Model, "--effort", o.Effort, "--json-schema", o.Schema}, nil
	default:
		return nil, ErrResearchProvider
	}
}

// RunResearch uses installed CLI authentication without reading credentials.
func RunResearch(ctx context.Context, prompt string, o ResearchOptions) (string, error) {
	bin, ok := resolveBinary([]string{o.Provider}, homePaths(".local/bin/"+o.Provider, "bin/"+o.Provider))
	if !ok {
		return "", ErrUnavailable
	}
	if err := os.MkdirAll(o.Directory, 0700); err != nil {
		return "", err
	}
	schemaPath, resultPath := filepath.Join(o.Directory, "schema.json"), filepath.Join(o.Directory, "result.json")
	args, err := researchArgs(o, schemaPath, resultPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(schemaPath, []byte(o.Schema), 0600); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = o.Directory
	cmd.Stdin = strings.NewReader(prompt)
	cmd.WaitDelay = 3 * time.Second
	var stderr limitedBuffer
	cmd.Stderr = &stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}
	var last, runError string
	scan := bufio.NewScanner(out)
	scan.Buffer(make([]byte, 65536), 8<<20)
	for scan.Scan() {
		var ev map[string]json.RawMessage
		if json.Unmarshal(scan.Bytes(), &ev) != nil {
			continue
		}
		var kind string
		_ = json.Unmarshal(ev["type"], &kind)
		if o.Progress != nil && (kind == "item.started" || kind == "assistant") {
			o.Progress("원자료를 확인하고 답변을 구성하고 있습니다.")
		}
		if kind == "error" || kind == "turn.failed" {
			runError = string(ev["message"])
			if runError == "" {
				runError = string(ev["error"])
			}
		}
		if kind == "result" {
			var isError bool
			_ = json.Unmarshal(ev["is_error"], &isError)
			if isError {
				_ = json.Unmarshal(ev["result"], &runError)
			} else if raw := ev["structured_output"]; len(raw) > 0 && string(raw) != "null" {
				last = string(raw)
			} else {
				_ = json.Unmarshal(ev["result"], &last)
			}
		}
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err := scan.Err(); err != nil {
		return "", err
	}
	if o.Provider == "codex" {
		if b, e := os.ReadFile(resultPath); e == nil {
			last = string(b)
		}
	}
	if waitErr != nil || runError != "" {
		if runError == "" {
			runError = strings.TrimSpace(stderr.String())
		}
		if runError == "" {
			runError = "CLI 실행 실패: 로그인 상태와 모델 접근 권한을 확인해 주세요."
		}
		return "", fmt.Errorf("%s: %s", o.Provider, limitText(runError, 1400))
	}
	if strings.TrimSpace(last) == "" {
		return "", errors.New("CLI에서 구조화된 조사 결과를 받지 못했습니다")
	}
	return last, nil
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(p []byte) (int, error) {
	n := len(p)
	if b.Len() < 8192 {
		_, _ = b.Buffer.Write(p[:min(len(p), 8192-b.Len())])
	}
	return n, nil
}
func limitText(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

var _ io.Writer = (*limitedBuffer)(nil)
