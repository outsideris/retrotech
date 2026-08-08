package assist

import (
	"strings"
	"testing"
)

func TestParseScriptMeta(t *testing.T) {
	sm, err := parseScriptMeta(`{"title":"2h. VCS: Git","id":"2h","description":"git 이야기"}`)
	if err != nil || sm.Title != "2h. VCS: Git" || sm.ID != "2h" || sm.Description != "git 이야기" {
		t.Errorf("plain JSON: %#v err %v", sm, err)
	}

	// A model that wraps the object in a ```json fence with surrounding prose.
	fenced := "결과입니다:\n```json\n{\"title\":\"T\",\"id\":\"\",\"description\":\"요약\"}\n```\n이상."
	sm, err = parseScriptMeta(fenced)
	if err != nil || sm.Title != "T" || sm.ID != "" || sm.Description != "요약" {
		t.Errorf("fenced: %#v err %v", sm, err)
	}

	if sm, _ := parseScriptMeta(`{"title":"  X  ","id":" 1a ","description":" d "}`); sm.Title != "X" || sm.ID != "1a" || sm.Description != "d" {
		t.Errorf("whitespace not trimmed: %#v", sm)
	}

	if _, err := parseScriptMeta("sorry, I can't do that"); err == nil {
		t.Error("non-JSON response should error")
	}
	if _, err := parseScriptMeta(`{"title":"","description":""}`); err == nil {
		t.Error("empty title and description should error")
	}
}

// Scripts head their title with "Episode" ("Episode 2i Subversion") but the
// published title starts at the episode id, so the label is dropped on import.
func TestNormalizeScriptTitle(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"the script label is dropped", "Episode 2i Subversion", "2i Subversion"},
		{"lowercase label too", "episode 2i Subversion", "2i Subversion"},
		{"a separator after the label goes with it", "Episode: 2i Subversion", "2i Subversion"},
		{"a dash separator too", "Episode - 2i Subversion", "2i Subversion"},
		{"surrounding whitespace is trimmed", "  Episode 2i Subversion  ", "2i Subversion"},
		{"a title without the label is untouched", "2h. VCS: Subversion", "2h. VCS: Subversion"},
		{"a word merely starting with episode is untouched", "Episodes of Subversion", "Episodes of Subversion"},
		{"the label mid-title is untouched", "2i. Episode Subversion", "2i. Episode Subversion"},
		{"a title that is only the label is kept", "Episode", "Episode"},
		{"empty stays empty", "", ""},
	}
	for _, tt := range tests {
		if got := normalizeScriptTitle(tt.in); got != tt.want {
			t.Errorf("%s: normalizeScriptTitle(%q) = %q, want %q", tt.name, tt.in, got, tt.want)
		}
	}

	// The whole point is the imported form: the title arrives label-free.
	sm, err := parseScriptMeta(`{"title":"Episode 2i Subversion","id":"2i","description":"요약"}`)
	if err != nil || sm.Title != "2i Subversion" || sm.ID != "2i" {
		t.Errorf("parseScriptMeta did not strip the label: %#v err %v", sm, err)
	}
}

func TestParseScriptMetaReferences(t *testing.T) {
	sm, err := parseScriptMeta(`{"title":"T","id":"","description":"d",
		"references":[{"title":"jQuery","url":"https://jquery.com/"}]}`)
	if err != nil || len(sm.References) != 1 || sm.References[0].Title != "jQuery" {
		t.Errorf("references not parsed: %#v err %v", sm, err)
	}
}

// TestReconcileRefs: the final reference list is exactly the extracted links in
// document order — model omissions fall back, model inventions are dropped.
func TestReconcileRefs(t *testing.T) {
	links := []Link{
		{Text: "John Resig", URL: "https://johnresig.com/"},
		{Text: "", URL: "https://jquery.com/"},
		{Text: "", URL: "https://unknown.example.com/x"},
	}
	refs := []ScriptRef{
		{Title: "jQuery", URL: "https://jquery.com/"},                  // out of order: fine
		{Title: "John Resig: Homepage", URL: "https://johnresig.com/"}, // titled
		{Title: "Invented", URL: "https://not-in-script.example.com/"}, // invented: dropped
		{Title: "  ", URL: "https://unknown.example.com/x"},            // blank title: fallback
	}
	got := reconcileRefs(links, refs)
	want := []ScriptRef{
		{Title: "John Resig: Homepage", URL: "https://johnresig.com/"},
		{Title: "jQuery", URL: "https://jquery.com/"},
		{Title: "https://unknown.example.com/x", URL: "https://unknown.example.com/x"}, // no title, no anchor → URL
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("ref %d: got %#v, want %#v", i, got[i], want[i])
		}
	}

	// Anchor-text fallback when the model skips a link entirely.
	got = reconcileRefs([]Link{{Text: "앵커", URL: "https://a.example.com/"}}, nil)
	if len(got) != 1 || got[0].Title != "앵커" {
		t.Errorf("anchor fallback: %#v", got)
	}
}

// TestAnalyzePromptIncludesLinksAndRules: every extracted URL must reach the
// prompt (the model can only title what it is shown), along with the naming
// rules; a script with no links must not ask for references at all.
func TestAnalyzePromptIncludesLinksAndRules(t *testing.T) {
	links := []Link{
		{Text: "John Resig", URL: "https://johnresig.com/"},
		{Text: "", URL: "https://en.wikipedia.org/wiki/JSONP"},
	}
	p := analyzePrompt("대본 본문", links)
	for _, l := range links {
		if !strings.Contains(p, l.URL) {
			t.Errorf("prompt missing URL %s", l.URL)
		}
	}
	for _, must := range []string{"references", "Wikipedia", "링크 목록", "앵커 텍스트"} {
		if !strings.Contains(p, must) {
			t.Errorf("prompt missing %q", must)
		}
	}

	if p := analyzePrompt("대본 본문", nil); strings.Contains(p, "references") {
		t.Error("link-less prompt should not mention references")
	}
}

func TestExtractJSONObject(t *testing.T) {
	if got := extractJSONObject(`prefix {"a":1} suffix`); got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
	if got := extractJSONObject("no json here"); got != "no json here" {
		t.Errorf("passthrough: %q", got)
	}
}
