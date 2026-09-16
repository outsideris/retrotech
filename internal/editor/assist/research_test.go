package assist

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestResearchArguments(t *testing.T) {
	for _, o := range []ResearchOptions{{Provider: "codex", Model: "gpt-5.6-sol", Effort: "high"}, {Provider: "claude", Model: "claude-sonnet-5", Effort: "high", Schema: `{"type":"object"}`}} {
		args, e := researchArgs(o, "schema.json", "result.json")
		if e != nil {
			t.Fatal(e)
		}
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "bypass") || strings.Contains(joined, "--bare") {
			t.Fatal("unsafe invocation")
		}
		if o.Provider == "codex" && (!strings.Contains(joined, `web_search="live"`) || !strings.Contains(joined, "--sandbox read-only")) {
			t.Fatal(joined)
		}
		if o.Provider == "claude" && (!strings.Contains(joined, "--safe-mode") || !strings.Contains(joined, "WebSearch,WebFetch")) {
			t.Fatal(joined)
		}
	}
	if _, e := researchArgs(ResearchOptions{Provider: "codex", Model: "gpt-6-astra", Effort: "high"}, "", ""); e == nil {
		t.Fatal("Astra not allowed for follow-up")
	}
	if _, e := researchArgs(ResearchOptions{Provider: "codex", Model: "gpt-5.6-sol", Effort: "invalid"}, "", ""); e == nil {
		t.Fatal("invalid effort accepted")
	}
}
func TestResearchCLIEnvelopes(t *testing.T) {
	for _, provider := range []string{"codex", "claude"} {
		t.Run(provider, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("PATH", binDir)
			script := "#!/bin/sh\n"
			if provider == "codex" {
				script += `while [ "$#" -gt 0 ]; do
if [ "$1" = '--output-last-message' ]; then shift; dest="$1"; fi
shift
done
printf '%s' '{"title":"structured"}' > "$dest"
printf '%s\n' '{"type":"item.started"}'
`
			} else {
				script += `printf '%s\n' '{"type":"result","is_error":false,"structured_output":{"title":"structured"}}'
`
			}
			os.WriteFile(filepath.Join(binDir, provider), []byte(script), 0755)
			model := "gpt-5.6-sol"
			if provider == "claude" {
				model = "claude-sonnet-5"
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			out, e := RunResearch(ctx, "Question", ResearchOptions{Provider: provider, Model: model, Effort: "high", Directory: t.TempDir(), Schema: `{"type":"object"}`})
			if e != nil {
				t.Fatal(e)
			}
			if out != `{"title":"structured"}` {
				t.Fatal(out)
			}
		})
	}
}
func TestResearchCLIErrors(t *testing.T) {
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nprintf '%s\\n' '{\"type\":\"result\",\"is_error\":true,\"result\":\"Not logged in\"}'\nexit 1\n"), 0755)
	_, e := RunResearch(context.Background(), "Q", ResearchOptions{Provider: "claude", Model: "claude-sonnet-5", Effort: "high", Directory: t.TempDir(), Schema: `{}`})
	if e == nil || !strings.Contains(e.Error(), "Not logged in") {
		t.Fatal(e)
	}
}
