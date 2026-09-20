package assist

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The full effort vocabulary the two CLIs use. A model may take a subset of it
// (or none), but nothing outside it.
var knownEfforts = map[string]bool{"low": true, "medium": true, "high": true, "xhigh": true, "max": true, "ultra": true}

func TestCatalogEntriesAreWellFormed(t *testing.T) {
	for _, p := range Providers() {
		ids, labels := map[string]bool{}, map[string]bool{}
		for _, m := range p.Models() {
			// "" is reserved: the sidebar prepends it as "use the CLI default".
			if m.ID == "" {
				t.Errorf("%s: a catalog model must have an id", p.Name())
			}
			if m.Label == "" {
				t.Errorf("%s: model %q has no label", p.Name(), m.ID)
			}
			if ids[m.ID] {
				t.Errorf("%s: duplicate model id %q", p.Name(), m.ID)
			}
			if labels[m.Label] {
				t.Errorf("%s: duplicate model label %q", p.Name(), m.Label)
			}
			ids[m.ID], labels[m.Label] = true, true

			seen := map[string]bool{}
			for _, e := range m.Efforts {
				if !knownEfforts[e] {
					t.Errorf("%s/%s: unknown effort %q", p.Name(), m.ID, e)
				}
				if seen[e] {
					t.Errorf("%s/%s: duplicate effort %q", p.Name(), m.ID, e)
				}
				seen[e] = true
			}
		}
	}
}

// The sidebar's dropdowns are only as current as this catalog, and the whole
// entry matters: the label is all the user sees, and the levels decide what the
// effort knob offers. Refreshing the CLIs' model list is a deliberate change
// that updates this expectation too.
func TestCatalogOffersTheCurrentModels(t *testing.T) {
	want := map[string][]Model{
		"claude": {
			{ID: "claude-opus-5", Label: "Opus 5", Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
			{ID: "claude-sonnet-5", Label: "Sonnet 5", Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
			{ID: "claude-fable-5-1", Label: "Fable 5.1", Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
			{ID: "claude-haiku-4-5-20251001", Label: "Haiku 4.5", Efforts: nil}, // takes no level
		},
		"codex": {
			{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol", Efforts: []string{"low", "medium", "high", "xhigh", "max", "ultra"}},
			{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra", Efforts: []string{"low", "medium", "high", "xhigh", "max", "ultra"}},
			{ID: "gpt-5.6-luna", Label: "GPT-5.6 Luna", Efforts: []string{"low", "medium", "high", "xhigh", "max"}},
			{ID: "gpt-5.5", Label: "GPT-5.5", Efforts: []string{"low", "medium", "high", "xhigh"}},
		},
		"gemini": nil, // the agy/gemini CLI exposes no model or effort flags
	}
	for _, p := range Providers() {
		if !reflect.DeepEqual(p.Models(), want[p.Name()]) {
			t.Errorf("%s catalog = %+v,\n\twant %+v", p.Name(), p.Models(), want[p.Name()])
		}
	}
	if len(want) != len(Providers()) {
		t.Errorf("a provider was added or removed without updating this expectation")
	}
}

func TestValidateOptions(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		opts     Options
		wantErr  string // "" = must be accepted
	}{
		{"cli default model and effort", "claude", Options{}, ""},
		{"cli default model takes any level a model offers", "claude", Options{Effort: "max"}, ""},
		{"cli default model rejects a level no model offers", "claude", Options{Effort: "ultra"}, "unsupported effort"},
		{"claude model with a supported level", "claude", Options{Model: "claude-opus-5", Effort: "xhigh"}, ""},
		{"claude model with codex's top level", "claude", Options{Model: "claude-sonnet-5", Effort: "ultra"}, "does not support effort"},
		{"model that takes no level", "claude", Options{Model: "claude-haiku-4-5-20251001"}, ""},
		{"model that takes no level, given one", "claude", Options{Model: "claude-haiku-4-5-20251001", Effort: "low"}, "takes no effort level"},
		{"retired claude alias", "claude", Options{Model: "opus"}, "unknown model"},

		{"codex model at ultra", "codex", Options{Model: "gpt-5.6-sol", Effort: "ultra"}, ""},
		{"codex model past its own ultra", "codex", Options{Model: "gpt-5.6-luna", Effort: "ultra"}, "does not support effort"},
		{"model this codex cannot run", "codex", Options{Model: "gpt-6-astra"}, "unknown model"},
		{"codex model past its top level", "codex", Options{Model: "gpt-5.5", Effort: "max"}, "does not support effort"},
		{"retired codex effort", "codex", Options{Model: "gpt-5.6-sol", Effort: "minimal"}, "does not support effort"},
		{"retired codex model", "codex", Options{Model: "gpt-5-codex"}, "unknown model"},
		{"codex model hidden from the account's catalog", "codex", Options{Model: "gpt-reserve"}, "unknown model"},

		{"gemini takes no knobs", "gemini", Options{}, ""},
		{"gemini given a model", "gemini", Options{Model: "gemini-3"}, "unknown model"},
		{"gemini given an effort", "gemini", Options{Effort: "high"}, "unsupported effort"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, ok := Find(tc.provider)
			if !ok {
				t.Fatalf("provider %q not registered", tc.provider)
			}
			err := Validate(p, tc.opts)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Errorf("Validate(%+v) = %v, want accepted", tc.opts, err)
			case tc.wantErr != "" && err == nil:
				t.Errorf("Validate(%+v) accepted, want %q", tc.opts, tc.wantErr)
			case tc.wantErr != "" && err != nil:
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("Validate(%+v) = %v, want it to mention %q", tc.opts, err, tc.wantErr)
				}
				if !strings.HasPrefix(err.Error(), tc.provider+":") {
					t.Errorf("error should name the provider: %v", err)
				}
			}
		})
	}
}

// A rejected model or effort must never reach the CLI, whatever the caller
// passes straight to Run. A fake CLI on PATH records the fact that it ran, so
// this fails loudly if validation ever stops guarding the exec.
func TestRunRejectsUncatalogedOptionsWithoutRunningTheCLI(t *testing.T) {
	for _, tc := range []struct {
		name     string
		provider string
		opts     Options
	}{
		{"retired claude alias", "claude", Options{Model: "opus", Effort: "high"}},
		{"claude at codex's top level", "claude", Options{Model: "claude-opus-5", Effort: "ultra"}},
		{"effort on a model that takes none", "claude", Options{Model: "claude-haiku-4-5-20251001", Effort: "high"}},
		{"retired codex model", "codex", Options{Model: "gpt-5-codex"}},
		{"codex past its model's top level", "codex", Options{Model: "gpt-5.5", Effort: "ultra"}},
		{"retired codex effort", "codex", Options{Model: "gpt-5.6-sol", Effort: "minimal"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			binDir := t.TempDir()
			t.Setenv("PATH", binDir)
			ran := filepath.Join(t.TempDir(), "ran")
			// `: > file` is a shell builtin: touch would not resolve, since
			// PATH is now the stub dir, and the marker would never appear.
			script := "#!/bin/sh\n: > " + ran + "\nprintf 'unexpected run'\n"
			if err := os.WriteFile(filepath.Join(binDir, tc.provider), []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}

			p, _ := Find(tc.provider)
			if _, _, err := p.Run(t.Context(), "unused", tc.opts); err == nil {
				t.Errorf("Run(%+v) should be rejected", tc.opts)
			}
			if _, err := os.Stat(ran); err == nil {
				t.Errorf("Run(%+v) executed the CLI instead of rejecting the options", tc.opts)
			}
		})
	}
}
