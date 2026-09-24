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
