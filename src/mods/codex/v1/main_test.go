package main

import (
	"strings"
	"testing"
)

func TestBuildStartCommandUsesRepoShellAndInlineCodexLaunch(t *testing.T) {
	cmd := buildStartCommand("/tmp/dialtone", "default", "medium", "gpt-5.4")
	if !strings.Contains(cmd, "cd '/tmp/dialtone'") {
		t.Fatalf("missing repo cd: %q", cmd)
	}
	if !strings.Contains(cmd, "develop '.#default'") {
		t.Fatalf("missing nix develop shell: %q", cmd)
	}
	if !strings.Contains(cmd, "command -v codex") {
		t.Fatalf("missing codex lookup: %q", cmd)
	}
	if !strings.Contains(cmd, "npx --yes @openai/codex") {
		t.Fatalf("missing npx fallback: %q", cmd)
	}
	if !strings.Contains(cmd, "exec codex -m '\"'\"'gpt-5.4'\"'\"' -a never -s danger-full-access") {
		t.Fatalf("missing codex args: %q", cmd)
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	quoted := shellQuote("/tmp/dialtone's")
	if quoted != "'/tmp/dialtone'\"'\"'s'" {
		t.Fatalf("unexpected quote result: %q", quoted)
	}
}

func TestBuildCodexExecCommandIncludesFallback(t *testing.T) {
	cmd := buildCodexExecCommand("gpt-5.4")
	if !strings.Contains(cmd, "exec codex") {
		t.Fatalf("missing direct codex exec: %q", cmd)
	}
	if !strings.Contains(cmd, "exec npx --yes @openai/codex") {
		t.Fatalf("missing npx exec: %q", cmd)
	}
}
