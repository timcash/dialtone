package main

import (
	"strings"
	"testing"
)

func TestNixDevelopCommand(t *testing.T) {
	t.Setenv("DIALTONE_NIX_ACTIVE", "")
	cmd := nixDevelopCommand("/tmp/dialtone", "go", "test", "./src/mods/repl/v1/...")
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "shell nixpkgs#bashInteractive nixpkgs#git nixpkgs#go_1_24 --command go test ./src/mods/repl/v1/...") {
		t.Fatalf("cmd args = %q", got)
	}
}
