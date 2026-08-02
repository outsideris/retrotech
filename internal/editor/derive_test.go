package editor

import "testing"

func TestDeriveDescription(t *testing.T) {
	tests := []struct {
		name  string
		intro string
		want  string
	}{
		{"plain text", "그냥 텍스트입니다.", "그냥 텍스트입니다.\n"},
		{
			"strips links",
			"[John Resig](https://johnresig.com/)이 공개한 [jQuery](https://jquery.com/) 이야기.",
			"John Resig이 공개한 jQuery 이야기.\n",
		},
		{
			"parens in url",
			"[Foo](https://en.wikipedia.org/wiki/Foo_(bar)) 설명.",
			"Foo 설명.\n",
		},
		{"image collapses to alt", "![커버](https://example.com/img.png) 텍스트.", "커버 텍스트.\n"},
		{"multi paragraph kept", "첫 문단.\n\n둘째 문단.", "첫 문단.\n\n둘째 문단.\n"},
		{"trailing whitespace trimmed", "텍스트.\n\n", "텍스트.\n"},
		{"empty", "", ""},
		{"whitespace only", "  \n\t", ""},
		// Re-deriving an already-derived description must not change it (Update
		// re-derives whenever the intro changed).
		{"idempotent", "이미 파생된 설명.\n", "이미 파생된 설명.\n"},
	}
	for _, tt := range tests {
		if got := deriveDescription(tt.intro); got != tt.want {
			t.Errorf("%s: deriveDescription(%q) = %q, want %q", tt.name, tt.intro, got, tt.want)
		}
	}
}
