package assist

import "testing"

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

func TestExtractJSONObject(t *testing.T) {
	if got := extractJSONObject(`prefix {"a":1} suffix`); got != `{"a":1}` {
		t.Errorf("got %q", got)
	}
	if got := extractJSONObject("no json here"); got != "no json here" {
		t.Errorf("passthrough: %q", got)
	}
}
