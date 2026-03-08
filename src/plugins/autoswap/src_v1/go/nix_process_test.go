package autoswap

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveNixProcessInvocationRun(t *testing.T) {
	dir := t.TempDir()
	nixPath := filepath.Join(dir, "nix")
	if err := os.WriteFile(nixPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake nix failed: %v", err)
	}
	prevPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+prevPath)
	defer func() { _ = os.Setenv("PATH", prevPath) }()

	got, sep, err := resolveNixProcessInvocation(manifestNix{
		Installable: "path:/home/tim/dialtone#robot-v2",
	}, func(v string) string { return v })
	if err != nil {
		t.Fatalf("resolveNixProcessInvocation(run) failed: %v", err)
	}
	if !sep {
		t.Fatalf("expected installable invocation to require arg separator")
	}
	if len(got) < 5 {
		t.Fatalf("unexpected argv length: %v", got)
	}
	if got[3] != "run" {
		t.Fatalf("expected nix run argv, got %v", got)
	}
}

func TestResolveNixProcessInvocationDevelop(t *testing.T) {
	dir := t.TempDir()
	nixPath := filepath.Join(dir, "nix")
	if err := os.WriteFile(nixPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake nix failed: %v", err)
	}
	prevPath := os.Getenv("PATH")
	_ = os.Setenv("PATH", dir+string(os.PathListSeparator)+prevPath)
	defer func() { _ = os.Setenv("PATH", prevPath) }()

	got, sep, err := resolveNixProcessInvocation(manifestNix{
		Develop: "path:/home/tim/dialtone",
		Command: []string{"bash", "-lc", "echo hi"},
	}, func(v string) string { return v })
	if err != nil {
		t.Fatalf("resolveNixProcessInvocation(develop) failed: %v", err)
	}
	if sep {
		t.Fatalf("did not expect nix develop invocation to require arg separator")
	}
	if len(got) < 8 {
		t.Fatalf("unexpected argv length: %v", got)
	}
	if got[3] != "develop" {
		t.Fatalf("expected nix develop argv, got %v", got)
	}
}
