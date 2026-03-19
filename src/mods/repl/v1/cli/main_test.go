package main

import (
	"strings"
	"testing"
)

func TestNixDevelopCommand(t *testing.T) {
	t.Setenv("DIALTONE_NIX_ACTIVE", "")
	cmd := nixDevelopCommand("/tmp/dialtone", "go", "test", "./src/mods/repl/v1/...")
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "develop path:/tmp/dialtone#repl-v1 --command go test ./src/mods/repl/v1/...") {
		t.Fatalf("cmd args = %q", got)
	}
}

func TestNixDevelopCommandUsesOfflineFlakeDevelop(t *testing.T) {
	t.Setenv("DIALTONE_NIX_ACTIVE", "")
	t.Setenv("DIALTONE_NIX_OFFLINE", "1")

	cmd := nixDevelopCommand("/tmp/dialtone", "go", "test", "./src/mods/repl/v1/...")
	got := strings.Join(cmd.Args, " ")

	if !strings.Contains(got, "--offline") {
		t.Fatalf("expected offline nix args, got %q", got)
	}
	if !strings.Contains(got, "develop path:/tmp/dialtone#repl-v1 --command") {
		t.Fatalf("expected repl-v1 flake develop in nix args, got %q", got)
	}
}
