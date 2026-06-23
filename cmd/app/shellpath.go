package main

// Login-shell PATH injection.
//
// A macOS GUI app launched by Finder/launchd inherits a bare PATH
// (/usr/bin:/bin:/usr/sbin:/sbin) — it does NOT source the user's shell
// profile. The assist CLIs (`claude` / `codex` / `gemini`) typically live in
// ~/.local/bin, Homebrew, an npm global dir, asdf, etc. Without help those
// binaries are invisible and every assist call fails as "unavailable". So at
// startup we ask a login shell what PATH it computes and adopt it.
//
// The shell exec is a stubbable var, so this is unit tested without depending
// on the host's real shell.

import (
	"os"
	"os/exec"
	"strings"
)

// runShellPath asks the given shell, run as a login shell, to print the PATH
// it ends up with. Overridable in tests.
var runShellPath = func(shell string) (string, error) {
	// -l: login shell (sources .zprofile/.bash_profile etc.); -c: run and exit.
	out, err := exec.Command(shell, "-lc", `printf %s "$PATH"`).Output()
	return string(out), err
}

// loginShellPath returns the PATH a login shell computes, or "" if it can't be
// determined.
func loginShellPath() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/zsh" // macOS default since Catalina
	}
	out, err := runShellPath(shell)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// injectLoginPath replaces the process PATH with the login shell's, so
// exec.LookPath (and thus the assist CLIs) resolve the same binaries the user's
// terminal does. A no-op when the lookup fails, leaving PATH untouched.
func injectLoginPath() {
	if p := loginShellPath(); p != "" {
		_ = os.Setenv("PATH", p)
	}
}
