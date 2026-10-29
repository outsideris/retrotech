package assist

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// The reader's picks are a curation decision recorded in
// docs/RESEARCH_WORKBENCH.md, pinned here so a refresh changes them on purpose.
// Each pick must also be one the installed CLIs run at every offered level.
func TestResearchOffersTheCuratedPicks(t *testing.T) {
	want := map[string][]string{
		"codex":  {"gpt-6.1-sol", "gpt-6-astra"},
		"claude": {"claude-sonnet-5-5", "claude-opus-5-5"},
	}
	if !reflect.DeepEqual(researchModels, want) {
		t.Fatalf("research models = %v, want %v", researchModels, want)
	}
	if want := []string{"medium", "high", "xhigh"}; !reflect.DeepEqual(researchEfforts, want) {
		t.Fatalf("research efforts = %v, want %v", researchEfforts, want)
	}
	for provider, models := range researchModels {
		p, ok := Find(provider)
		if !ok {
			t.Fatalf("the reader offers %s, which has no Assist catalog", provider)
		}
		for _, model := range models {
			for _, effort := range researchEfforts {
				if e := Validate(p, Options{Model: model, Effort: effort}); e != nil {
					t.Errorf("the reader offers what the CLI catalog rejects: %v", e)
				}
			}
		}
	}
}

func TestResearchArguments(t *testing.T) {
	for provider, models := range researchModels {
		for _, model := range models {
			for _, effort := range researchEfforts {
				args, e := researchArgs(ResearchOptions{Provider: provider, Model: model, Effort: effort, Schema: `{"type":"object"}`}, "schema.json", "result.json")
				if e != nil {
					t.Fatal(e)
				}
				joined := strings.Join(args, " ")
				if strings.Contains(joined, "bypass") || strings.Contains(joined, "--bare") {
					t.Fatal("unsafe invocation")
				}
				need := []string{"--model " + model + " "}
				if provider == "codex" {
					need = append(need, `web_search="live"`, "--sandbox read-only", "features.multi_agent=false", `model_reasoning_effort="`+effort+`"`)
				} else {
					need = append(need, "--safe-mode", "--tools WebSearch,WebFetch", "--effort "+effort+" ")
				}
				for _, n := range need {
					if !strings.Contains(joined, n) {
						t.Errorf("%s %s/%s: missing %q in %s", provider, model, effort, n, joined)
					}
				}
			}
		}
	}
}

func TestResearchRejectsWhatTheReaderDoesNotOffer(t *testing.T) {
	for _, tc := range []struct {
		name                    string
		provider, model, effort string
		want                    error
	}{
		{"efficient codex tier", "codex", "gpt-6-luna", "high", ErrResearchModel},
		{"writing and visual codex tier", "codex", "gpt-6.1-terra", "medium", ErrResearchModel},
		{"previous codex default", "codex", "gpt-6-sol", "high", ErrResearchModel},
		{"older codex default", "codex", "gpt-5.6-sol", "high", ErrResearchModel},
		{"older codex quick check", "codex", "gpt-5.6-terra", "medium", ErrResearchModel},
		{"Fable may bill credits unasked", "claude", "claude-fable-5-1", "high", ErrResearchModel},
		{"fastest Claude tier", "claude", "claude-haiku-5-5", "high", ErrResearchModel},
		{"previous Claude alternative", "claude", "claude-sonnet-5", "high", ErrResearchModel},
		{"previous Claude harder pick", "claude", "claude-opus-5", "xhigh", ErrResearchModel},
		{"the other CLI's model", "codex", "claude-opus-5-5", "high", ErrResearchModel},
		{"CLI default model", "codex", "", "high", ErrResearchModel},
		{"CLI without a research mode", "gemini", "gemini-2.5-pro", "high", ErrResearchProvider},
		{"too shallow for sources", "codex", "gpt-6.1-sol", "low", ErrResearchEffort},
		{"prone to overthinking", "claude", "claude-opus-5-5", "max", ErrResearchEffort},
		{"delegates to sub-agents", "codex", "gpt-6-astra", "ultra", ErrResearchEffort},
		{"no effort", "claude", "claude-sonnet-5-5", "", ErrResearchEffort},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if e := ValidateResearch(tc.provider, tc.model, tc.effort); !errors.Is(e, tc.want) {
				t.Errorf("ValidateResearch = %v, want %v", e, tc.want)
			}
			if _, e := researchArgs(ResearchOptions{Provider: tc.provider, Model: tc.model, Effort: tc.effort}, "", ""); !errors.Is(e, tc.want) {
				t.Errorf("researchArgs = %v, want %v", e, tc.want)
			}
		})
	}
}

// A caller that skips the reader's own check still cannot hand a CLI a model
// outside the list — Fable here could bill usage credits without asking. The
// fake CLI records that it ran, so this fails if validation stops guarding exec.
func TestResearchRejectsWithoutRunningTheCLI(t *testing.T) {
	for _, o := range []ResearchOptions{
		{Provider: "claude", Model: "claude-fable-5-1", Effort: "high"},
		{Provider: "codex", Model: "gpt-6-luna", Effort: "high"},
		{Provider: "codex", Model: "gpt-6-astra", Effort: "ultra"},
	} {
		t.Run(o.Model+"/"+o.Effort, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("PATH", binDir)
			ran := filepath.Join(t.TempDir(), "ran")
			// `: > file` is a shell builtin; touch would not resolve on this PATH.
			if e := os.WriteFile(filepath.Join(binDir, o.Provider), []byte("#!/bin/sh\n: > "+ran+"\n"), 0755); e != nil {
				t.Fatal(e)
			}
			o.Directory, o.Schema = t.TempDir(), `{}`
			if _, e := RunResearch(t.Context(), "Q", o); e == nil {
				t.Error("accepted an option the reader does not offer")
			}
			if _, e := os.Stat(ran); e == nil {
				t.Error("ran the CLI instead of rejecting the option")
			}
		})
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
			model := "gpt-6.1-sol"
			if provider == "claude" {
				model = "claude-sonnet-5-5"
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
	_, e := RunResearch(context.Background(), "Q", ResearchOptions{Provider: "claude", Model: "claude-sonnet-5-5", Effort: "high", Directory: t.TempDir(), Schema: `{}`})
	if e == nil || !strings.Contains(e.Error(), "Not logged in") {
		t.Fatal(e)
	}
}
