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

// The feed ships HTML (podcast apps render <description> as HTML, so plain
// newlines vanish), and the editor stores that HTML in the episode file so the
// markdown shows exactly what subscribers receive.
func TestDeriveFeedDescription(t *testing.T) {
	tests := []struct {
		name         string
		description  string
		description2 string
		want         string
	}{
		{
			name:        "line breaks inside the description survive as <br/>",
			description: "첫 문장.\n둘째 문장.\n",
			want:        "<p>첫 문장.<br/>둘째 문장.</p>\n",
		},
		{
			name:         "description2 becomes its own paragraph",
			description:  "요약.\n",
			description2: "레퍼런스는 홈페이지 참고:\nhttps://retrotech.outsider.dev/episodes/2h\n",
			want: "<p>요약.</p><p>레퍼런스는 홈페이지 참고:<br/>" +
				`<a href="https://retrotech.outsider.dev/episodes/2h">https://retrotech.outsider.dev/episodes/2h</a></p>` + "\n",
		},
		{
			name:         "a blank line inside description2 splits paragraphs",
			description:  "요약.\n",
			description2: "레퍼런스는 홈페이지 참고:\n\nMusic from #Uppbeat\nLicense code: X\n",
			want:         "<p>요약.</p><p>레퍼런스는 홈페이지 참고:</p><p>Music from #Uppbeat<br/>License code: X</p>\n",
		},
		{"no description2", "요약.\n", "", "<p>요약.</p>\n"},
		{"empty stays empty", "", "", ""},
		{"whitespace only stays empty", "\n\n", "", ""},
	}
	for _, tt := range tests {
		if got := deriveFeedDescription(tt.description, tt.description2); got != tt.want {
			t.Errorf("%s: deriveFeedDescription(%q, %q) = %q, want %q",
				tt.name, tt.description, tt.description2, got, tt.want)
		}
	}
}
