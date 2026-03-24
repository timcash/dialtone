package modcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOutputPathUsesMirroredBinLayout(t *testing.T) {
	repoRoot := t.TempDir()
	path, err := BuildOutputPath(repoRoot, "shell", "v1", "shell")
	if err != nil {
		t.Fatalf("BuildOutputPath returned error: %v", err)
	}
	want := filepath.Join(repoRoot, "bin", "mods", "shell", "v1", "shell")
	if path != want {
		t.Fatalf("BuildOutputPath = %q, want %q", path, want)
	}
}

func TestNixDevelopCommandUsesCurrentEnvWhenAlreadyActive(t *testing.T) {
	t.Setenv("DIALTONE_NIX_ACTIVE", "1")
	t.Setenv("DIALTONE_GO_BIN", "/tmp/go-bin")

	cmd := NixDevelopCommand("/tmp/dialtone", "default", "go", "test", "./mods/shell/v1/...")
	if got := strings.Join(cmd.Args, " "); got != "/tmp/go-bin test ./mods/shell/v1/..." {
		t.Fatalf("unexpected active-shell command: %q", got)
	}
}

func TestNixDevelopCommandWrapsThroughNixWhenInactive(t *testing.T) {
	t.Setenv("DIALTONE_NIX_ACTIVE", "")
	cmd := NixDevelopCommand("/tmp/dialtone", "default", "go", "test", "./mods/shell/v1/...")
	got := strings.Join(cmd.Args, " ")
	if !strings.Contains(got, "develop path:/tmp/dialtone#default --command go test ./mods/shell/v1/...") {
		t.Fatalf("unexpected nix develop args: %q", got)
	}
}

func TestFindRepoRootUsesEnvironmentOverride(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, "src"), 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "dialtone_mod"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write dialtone_mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "src", "go.mod"), []byte("module dialtone/dev\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	t.Setenv("DIALTONE_REPO_ROOT", repoRoot)

	got, err := FindRepoRoot()
	if err != nil {
		t.Fatalf("FindRepoRoot returned error: %v", err)
	}
	if got != repoRoot {
		t.Fatalf("FindRepoRoot = %q, want %q", got, repoRoot)
	}
}

func TestCurrentTmuxTargetPrefersExplicitTarget(t *testing.T) {
	lookups := 0
	got := CurrentTmuxTarget("codex-view:0:1", "%31", func(string) (string, error) {
		lookups++
		return "unexpected", nil
	})
	if got != "codex-view:0:1" {
		t.Fatalf("CurrentTmuxTarget explicit target = %q, want %q", got, "codex-view:0:1")
	}
	if lookups != 0 {
		t.Fatalf("expected explicit target to bypass tmux lookup, got %d lookups", lookups)
	}
}

func TestCurrentTmuxTargetFallsBackToLookup(t *testing.T) {
	got := CurrentTmuxTarget("", "%31", func(paneID string) (string, error) {
		if paneID != "%31" {
			t.Fatalf("lookup paneID = %q, want %q", paneID, "%31")
		}
		return "codex-view:0:1", nil
	})
	if got != "codex-view:0:1" {
		t.Fatalf("CurrentTmuxTarget lookup target = %q, want %q", got, "codex-view:0:1")
	}
}

func TestNormalizeOptionalPathArgKeepsBlankEmpty(t *testing.T) {
	if got := NormalizeOptionalPathArg(""); got != "" {
		t.Fatalf("NormalizeOptionalPathArg(\"\") = %q, want empty string", got)
	}
	if got := NormalizeOptionalPathArg("   "); got != "" {
		t.Fatalf("NormalizeOptionalPathArg(blank) = %q, want empty string", got)
	}
}

func TestNormalizeOptionalPathArgCleansExplicitPath(t *testing.T) {
	if got := NormalizeOptionalPathArg("./mods/test/v1/../v1"); got != "mods/test/v1" {
		t.Fatalf("NormalizeOptionalPathArg cleaned path = %q, want %q", got, "mods/test/v1")
	}
}
