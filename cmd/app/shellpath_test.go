package main

import (
	"errors"
	"os"
	"testing"
)

func TestInjectLoginPathAdoptsShellPath(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	orig := runShellPath
	t.Cleanup(func() { runShellPath = orig })
	runShellPath = func(shell string) (string, error) {
		return "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin\n", nil
	}

	injectLoginPath()

	if got := os.Getenv("PATH"); got != "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin" {
		t.Errorf("PATH = %q", got)
	}
}

func TestInjectLoginPathLeavesPathOnFailure(t *testing.T) {
	t.Setenv("PATH", "/usr/bin:/bin")
	orig := runShellPath
	t.Cleanup(func() { runShellPath = orig })
	runShellPath = func(shell string) (string, error) {
		return "", errors.New("no shell")
	}

	injectLoginPath()

	if got := os.Getenv("PATH"); got != "/usr/bin:/bin" {
		t.Errorf("PATH should be untouched on failure, got %q", got)
	}
}
