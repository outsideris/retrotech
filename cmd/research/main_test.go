package main

import (
	"strings"
	"testing"
)

func TestLauncherDoesNotOpenAnotherWindow(t *testing.T) {
	s := launcherScript("/Applications/My Tools/research", "/Users/example/host's report", "/Users/example/skill")
	if strings.Contains(s, "-open") || !strings.Contains(s, "exec '/Applications/My Tools/research'") || !strings.Contains(s, `host'\''s report`) {
		t.Fatal("launcher must start only the connector and quote its paths")
	}
}

// The skill was lost once while it lived only in ~/.codex/skills on one Mac;
// the default now names the link in Codex's user skill folder.
func TestDefaultSkillIsTheCodexUserSkillLink(t *testing.T) {
	if got := defaultSkillDir("/Users/example"); got != "/Users/example/.agents/skills/research-retrotech" {
		t.Fatalf("default skill directory = %s", got)
	}
}
