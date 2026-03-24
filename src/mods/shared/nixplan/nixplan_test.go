package nixplan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"dialtone/dev/internal/modstate"
)

func TestBuildPlanUsesLaunchConfigAndDBPackages(t *testing.T) {
	repoRoot := t.TempDir()
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()
	if err := modstate.EnsureSchema(db); err != nil {
		t.Fatalf("EnsureSchema returned error: %v", err)
	}
	if _, err := db.Exec(`insert into mod_launch_configs(mod_name, mod_version, flake_shell, updated_at) values('ssh','v1','ssh-v1','now')`); err != nil {
		t.Fatalf("insert launch config: %v", err)
	}
	if _, err := db.Exec(`insert into mod_nix_packages(mod_name, mod_version, selector, package_ref, updated_at) values('ssh','v1','all','nixpkgs#expect','now')`); err != nil {
		t.Fatalf("insert nix package: %v", err)
	}

	plan, err := BuildPlan(db, repoRoot, "ssh", "v1", runtime.GOOS)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan.FlakeShell != "ssh-v1" {
		t.Fatalf("unexpected flake shell: %+v", plan)
	}
	if !contains(plan.Packages, "nixpkgs#expect") {
		t.Fatalf("expected db package in plan: %+v", plan)
	}
	if !contains(plan.Packages, "nixpkgs#go_1_25") {
		t.Fatalf("expected default go package in plan: %+v", plan)
	}
}

func TestBuildPlanFallsBackToManifestPackages(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "demo", "v1", "nix.packages"), "all:nixpkgs#sqlite\nlinux:nixpkgs#strace\ndarwin:nixpkgs#ghostty\n")

	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	plan, err := BuildPlan(db, repoRoot, "demo", "v1", "darwin")
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if !contains(plan.Packages, "nixpkgs#sqlite") {
		t.Fatalf("expected all-selector package in plan: %+v", plan)
	}
	if !contains(plan.Packages, "nixpkgs#ghostty") {
		t.Fatalf("expected darwin-selector package in plan: %+v", plan)
	}
	if contains(plan.Packages, "nixpkgs#strace") {
		t.Fatalf("did not expect linux-selector package in darwin plan: %+v", plan)
	}
}

func TestBuildPlanUsesFallbackFlakeShells(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	plan, err := BuildPlan(db, t.TempDir(), "demo", "v1", runtime.GOOS)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan.FlakeShell != "default" {
		t.Fatalf("unexpected fallback flake shell: %+v", plan)
	}
}

func TestBuildPlanUsesSharedDefaultShellForSSH(t *testing.T) {
	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	plan, err := BuildPlan(db, t.TempDir(), "ssh", "v1", runtime.GOOS)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if plan.FlakeShell != "default" {
		t.Fatalf("expected shared default shell for ssh, got %+v", plan)
	}
}

func TestBuildPlanDeduplicatesSharedPackages(t *testing.T) {
	repoRoot := t.TempDir()
	writeFile(t, filepath.Join(repoRoot, "src", "mods", "demo", "v1", "nix.packages"), "all:nixpkgs#git\nall:nixpkgs#go_1_25\nall:nixpkgs#expect\nall:nixpkgs#expect\n")

	db, err := modstate.Open(filepath.Join(t.TempDir(), "state.sqlite"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	defer db.Close()

	plan, err := BuildPlan(db, repoRoot, "demo", "v1", runtime.GOOS)
	if err != nil {
		t.Fatalf("BuildPlan returned error: %v", err)
	}
	if !contains(plan.Packages, "nixpkgs#expect") {
		t.Fatalf("expected expect package in plan: %+v", plan)
	}
	if count(plan.Packages, "nixpkgs#git") != 1 {
		t.Fatalf("expected shared git package exactly once: %+v", plan)
	}
	if count(plan.Packages, "nixpkgs#go_1_25") != 1 {
		t.Fatalf("expected shared go package exactly once: %+v", plan)
	}
	if count(plan.Packages, "nixpkgs#expect") != 1 {
		t.Fatalf("expected expect package exactly once: %+v", plan)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func count(values []string, target string) int {
	total := 0
	for _, value := range values {
		if value == target {
			total++
		}
	}
	return total
}
