package assist

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvidersAndFind(t *testing.T) {
	if got := len(Providers()); got != 3 {
		t.Fatalf("want 3 providers, got %d", got)
	}
	for _, name := range []string{"claude", "codex", "gemini"} {
		if _, ok := Find(name); !ok {
			t.Errorf("Find(%q) not found", name)
		}
	}
	if _, ok := Find("nope"); ok {
		t.Error("Find(nope) should be false")
	}
}

func TestParseClaude(t *testing.T) {
	out, err := parseClaude([]byte(`{"type":"result","is_error":false,"result":"hello world"}`))
	if err != nil || out != "hello world" {
		t.Errorf("normal envelope: got %q, err %v", out, err)
	}

	if _, err := parseClaude([]byte(`{"is_error":true,"result":"boom"}`)); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("is_error: %v", err)
	}

	// Not the expected JSON → fall back to raw stdout.
	if out, err := parseClaude([]byte("just plain text")); err != nil || out != "just plain text" {
		t.Errorf("raw fallback: got %q, err %v", out, err)
	}

	if _, err := parseClaude([]byte("   ")); err == nil {
		t.Error("empty output should error")
	}
}

func TestParseCodex(t *testing.T) {
	stream := strings.Join([]string{
		`{"type":"thread.started"}`,
		`a non-JSON warning line — must be tolerated`,
		`{"type":"item.completed","item":{"type":"reasoning","text":"thinking..."}}`,
		`{"type":"item.completed","item":{"type":"agent_message","text":"final answer"}}`,
		`{"type":"turn.completed","usage":{"input_tokens":10}}`,
	}, "\n")
	if out, err := parseCodex([]byte(stream)); err != nil || out != "final answer" {
		t.Errorf("agent message: got %q, err %v", out, err)
	}

	if _, err := parseCodex([]byte(`{"type":"error","message":"rate limited"}`)); err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Errorf("error event: %v", err)
	}

	if _, err := parseCodex([]byte(`{"type":"turn.completed"}`)); err == nil {
		t.Error("stream with no agent message should error")
	}
}

func TestResolveBinary(t *testing.T) {
	// Nothing on PATH and a non-existent fallback → not resolved.
	if _, ok := resolveBinary([]string{"retrotech-no-such-binary-xyz"}, []string{"/no/such/path"}); ok {
		t.Error("missing binary with missing fallback should not resolve")
	}

	// Falls back to an existing path when the names aren't on PATH (covers the
	// `agy`/non-PATH install case).
	fake := filepath.Join(t.TempDir(), "fakecli")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got, ok := resolveBinary([]string{"retrotech-no-such-binary-xyz"}, []string{"/no/such/path", fake}); !ok || got != fake {
		t.Errorf("fallback resolve: got %q, ok=%v", got, ok)
	}
}
