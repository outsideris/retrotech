package assist

import "fmt"

// Model is one selectable entry in the Assist sidebar's model dropdown: the id
// handed to the CLI's --model flag, the label the sidebar shows, and the
// reasoning-effort levels that model accepts. Efforts is nil for a model that
// takes no effort flag at all, so the sidebar hides the knob instead of
// offering a level the model would ignore.
type Model struct {
	ID      string   `json:"id"`
	Label   string   `json:"label"`
	Efforts []string `json:"efforts"`
}

// Effort levels, named once so the per-model sets below read as differences.
var (
	effortsToMax   = []string{"low", "medium", "high", "xhigh", "max"}
	effortsToUltra = []string{"low", "medium", "high", "xhigh", "max", "ultra"}
)

// claudeModels / codexModels are the models each CLI can run today, in the
// order the CLI's own model picker lists them. An empty model id ("use
// whatever the CLI is configured for") is not listed here — the sidebar
// prepends that choice itself and validateOptions accepts it against the
// provider's union of levels.
//
// Kept by hand and checked against the installed CLIs on 2026-09-30 —
// claude 2.1.284 and codex 0.159.0. Refresh it after a CLI upgrade; the CLIs
// do not report a machine-readable list this code could read instead. A new
// model can outrun the installed CLI: codex 0.146.0 refused the GPT-6 models
// ("not supported when using Codex with a ChatGPT account") and claude 2.1.278
// flagged Opus 5.5 as an unrecognized model, so upgrade the CLI first.
//
//   - claude: ids, display names and effort support come from the CLI's own
//     model registry. Only the current lineup is offered; the CLI's picker
//     now labels Opus 5 and Sonnet 5 "Previous … version". Every 5-family
//     model takes low…max. Haiku 4.5 takes no level at all: the CLI accepts
//     --effort for it and silently drops it, so offering one would only
//     promise thinking that never happens.
//   - codex: the account's catalog (~/.codex/models_cache.json) lists the
//     models and the levels each accepts; hidden entries (gpt-reserve,
//     codex-auto-review) are not offered. GPT-5.5 is left out although the
//     catalog still lists it: it retires on 2026-10-14, with GPT-5.6 Sol
//     named as its replacement. Neither the CLI nor the API enforces those
//     levels any more — GPT-6 Luna given ultra ran without an error — so, as
//     with Haiku, the catalog has to hold each model's ceiling itself.
var (
	claudeModels = []Model{
		{ID: "claude-opus-5-5", Label: "Opus 5.5", Efforts: effortsToMax},
		{ID: "claude-sonnet-5-5", Label: "Sonnet 5.5", Efforts: effortsToMax},
		{ID: "claude-fable-5-1", Label: "Fable 5.1", Efforts: effortsToMax},
		{ID: "claude-haiku-4-5-20251001", Label: "Haiku 4.5", Efforts: nil},
	}

	codexModels = []Model{
		{ID: "gpt-6-astra", Label: "GPT-6 Astra", Efforts: effortsToUltra},
		{ID: "gpt-6-sol", Label: "GPT-6 Sol", Efforts: effortsToUltra},
		{ID: "gpt-6-luna", Label: "GPT-6 Luna", Efforts: effortsToMax},
		{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol", Efforts: effortsToUltra},
		{ID: "gpt-5.6-terra", Label: "GPT-5.6 Terra", Efforts: effortsToUltra},
		{ID: "gpt-5.6-luna", Label: "GPT-5.6 Luna", Efforts: effortsToMax},
	}
)

// findModel returns the catalog entry for id.
func findModel(models []Model, id string) (Model, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return Model{}, false
}

// Validate reports whether a provider can be asked to run with these options.
// The HTTP layer calls it before dispatching so an unknown model or an effort
// the chosen model doesn't take is a request error, not a CLI failure — and
// the answer is the same whether or not that CLI is installed here.
func Validate(p Provider, o Options) error {
	return validateOptions(p.Name(), p.Models(), o)
}

// validateOptions rejects a model or effort the CLI would only fail on later,
// with a message naming what is wrong. An empty model means "whatever the CLI
// is configured for", whose effort support we can't know here, so its effort is
// checked against every level any of the provider's models accepts.
func validateOptions(provider string, models []Model, o Options) error {
	if o.Model == "" {
		if o.Effort != "" && !supportedByAny(models, o.Effort) {
			return fmt.Errorf("%s: unsupported effort %q", provider, o.Effort)
		}
		return nil
	}
	m, ok := findModel(models, o.Model)
	if !ok {
		return fmt.Errorf("%s: unknown model %q", provider, o.Model)
	}
	if o.Effort == "" {
		return nil
	}
	if len(m.Efforts) == 0 {
		return fmt.Errorf("%s: model %q takes no effort level", provider, o.Model)
	}
	for _, e := range m.Efforts {
		if e == o.Effort {
			return nil
		}
	}
	return fmt.Errorf("%s: model %q does not support effort %q", provider, o.Model, o.Effort)
}

func supportedByAny(models []Model, effort string) bool {
	for _, m := range models {
		for _, e := range m.Efforts {
			if e == effort {
				return true
			}
		}
	}
	return false
}
