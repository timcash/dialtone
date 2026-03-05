package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestSSHParseArgs(t *testing.T) {
	opts, err := parseArgs([]string{"--host", "gold", "--user", "user", "--port", "2022", "--command", "echo hello", "--dry-run"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if opts.host != "gold" {
		t.Fatalf("expected host gold, got %q", opts.host)
	}
	if opts.user != "user" {
		t.Fatalf("expected user user, got %q", opts.user)
	}
	if opts.port != "2022" {
		t.Fatalf("expected port 2022, got %q", opts.port)
	}
	if opts.command != "echo hello" {
		t.Fatalf("expected command echo hello, got %q", opts.command)
	}
	if !opts.dryRun {
		t.Fatalf("expected dry-run true")
	}
}

func TestSSHParseArgsPositionalHost(t *testing.T) {
	opts, err := parseArgs([]string{"--host", "wsl", "--command", "uptime"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if opts.host != "wsl" {
		t.Fatalf("expected host wsl, got %q", opts.host)
	}
	if opts.command != "uptime" {
		t.Fatalf("expected command uptime, got %q", opts.command)
	}
}

func TestSSHParseArgsHostFallbackFromRunPrefix(t *testing.T) {
	tmp := t.TempDir()
	nixPath := filepath.Join(tmp, "fake-nix")
	if err := os.WriteFile(nixPath, []byte("#!/bin/sh\necho test\n"), 0o755); err != nil {
		t.Fatalf("write fake nix failed: %v", err)
	}
	meshPath := filepath.Join(tmp, "env", "mesh.json")

	if err := os.MkdirAll(filepath.Dir(meshPath), 0o755); err != nil {
		t.Fatalf("mkdir env dir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write dialtone entrypoint failed: %v", err)
	}
	if err := os.WriteFile(meshPath, []byte(`[{"name":"gold","aliases":["gold"],"host":"10.0.0.1","user":"alice","port":"22"}]`), 0o644); err != nil {
		t.Fatalf("write mesh config failed: %v", err)
	}

	oldNixBin := os.Getenv("NIX_BIN")
	oldRepoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	_ = os.Setenv("NIX_BIN", nixPath)
	_ = os.Setenv("DIALTONE_REPO_ROOT", tmp)
	defer func() {
		_ = os.Setenv("NIX_BIN", oldNixBin)
		_ = os.Setenv("DIALTONE_REPO_ROOT", oldRepoRoot)
	}()

	output := captureStdout(t, func() {
		if err := run([]string{"run", "--host", "gold", "--dry-run"}); err != nil {
			t.Fatalf("run with run-prefix failed: %v", err)
		}
	})
	if !strings.Contains(output, "nix command:") {
		t.Fatalf("run with run prefix did not emit dry-run command: %q", output)
	}
}

func TestSSHResolveMeshNodeFromConfig(t *testing.T) {
	payload := `[
		{"name":"wsl","aliases":["wsl","legion-wsl-1"],"user":"user","host":"192.168.4.52","port":"22"},
		{"name":"gold","aliases":["gold"],"user":"user","host":"192.168.4.53","port":"22"}
	]`
	var nodes []meshNode
	if err := json.Unmarshal([]byte(payload), &nodes); err != nil {
		t.Fatalf("invalid mesh payload: %v", err)
	}

	got, ok := resolveMeshNodeFromConfig(nodes, "LEGION-WSL-1.")
	if !ok {
		t.Fatalf("expected alias resolution for LEGION-WSL-1.")
	}
	if got.Name != "wsl" || got.Host != "192.168.4.52" {
		t.Fatalf("resolved wrong node: %#v", got)
	}
}

func TestSSHBuildCommandUsesNodeDefaults(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	opts, err := parseArgs([]string{"--host", "gold"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	oldNixBin := os.Getenv("NIX_BIN")
	_ = os.Setenv("NIX_BIN", "/opt/nix/bin/nix")
	defer func() {
		_ = os.Setenv("NIX_BIN", oldNixBin)
	}()

	cmd, err := buildSSHCommand(opts, node)
	if err != nil {
		t.Fatalf("buildSSHCommand failed: %v", err)
	}
	if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-p 2223") {
		t.Fatalf("expected port 2223 from mesh node, got args %q", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(cmd.Args[len(cmd.Args)-1]), "@example.com") {
		t.Fatalf("expected default user target, got target arg %q", cmd.Args[len(cmd.Args)-1])
	}
}

func TestSSHBuildCommandRespectsOverrides(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	opts, err := parseArgs([]string{"--host", "gold", "--user", "alice", "--port", "2200", "--command", "echo hi", "--nixpkgs-url", "nix://example"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	oldNixBin := os.Getenv("NIX_BIN")
	_ = os.Setenv("NIX_BIN", "/opt/nix/bin/nix")
	defer func() {
		_ = os.Setenv("NIX_BIN", oldNixBin)
	}()

	cmd, err := buildSSHCommand(opts, node)
	if err != nil {
		t.Fatalf("buildSSHCommand failed: %v", err)
	}
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "-p 2200") {
		t.Fatalf("expected CLI port override to win, got %q", joined)
	}
	if !strings.HasSuffix(strings.TrimSpace(cmd.Args[len(cmd.Args)-2]), "alice@example.com") {
		t.Fatalf("expected CLI user override, got %q", cmd.Args[len(cmd.Args)-2])
	}
	if got := cmd.Args[len(cmd.Args)-1]; got != "echo hi" {
		t.Fatalf("expected command arg, got %q", got)
	}
}

func TestSSHBuildCommandPrefersTailnetHostCandidate(t *testing.T) {
	node := meshNode{
		Name:           "gold",
		Host:           "10.0.0.9",
		HostCandidates: []string{"192.168.1.9", "gold.shad-artichoke.ts.net"},
		User:           "user",
		Port:           "22",
	}
	oldNixBin := os.Getenv("NIX_BIN")
	_ = os.Setenv("NIX_BIN", "/opt/nix/bin/nix")
	defer func() {
		_ = os.Setenv("NIX_BIN", oldNixBin)
	}()

	cmd, err := buildSSHCommand(sshOptions{}, node)
	if err != nil {
		t.Fatalf("buildSSHCommand failed: %v", err)
	}
	if !strings.HasSuffix(cmd.Args[len(cmd.Args)-1], "user@gold.shad-artichoke.ts.net") {
		t.Fatalf("expected tailnet host to be selected first, got target %q", cmd.Args[len(cmd.Args)-1])
	}
}

func TestSSHLocateNixBinaryEnvOverride(t *testing.T) {
	tmp := t.TempDir()
	fakeNix := filepath.Join(tmp, "nix")
	if err := os.WriteFile(fakeNix, []byte("#!/bin/sh\necho test\n"), 0o755); err != nil {
		t.Fatalf("write fake nix failed: %v", err)
	}

	old := os.Getenv("NIX_BIN")
	_ = os.Setenv("NIX_BIN", fakeNix)
	defer func() { _ = os.Setenv("NIX_BIN", old) }()

	got, err := locateNixBinary()
	if err != nil {
		t.Fatalf("locateNixBinary env override failed: %v", err)
	}
	if got != fakeNix {
		t.Fatalf("expected %q, got %q", fakeNix, got)
	}
}

func TestSSHLocateNixBinaryHomeFallback(t *testing.T) {
	tmp := t.TempDir()
	fakeHome := filepath.Join(tmp, "home")
	nixPath := filepath.Join(fakeHome, ".nix-profile/bin/nix")
	if err := os.MkdirAll(filepath.Dir(nixPath), 0o755); err != nil {
		t.Fatalf("mkdir home profile failed: %v", err)
	}
	if err := os.WriteFile(nixPath, []byte("#!/bin/sh\necho test\n"), 0o755); err != nil {
		t.Fatalf("write fallback nix failed: %v", err)
	}

	oldHome := os.Getenv("HOME")
	oldPath := os.Getenv("PATH")
	oldNix := os.Getenv("NIX_BIN")
	_ = os.Setenv("HOME", fakeHome)
	_ = os.Setenv("NIX_BIN", "")
	_ = os.Setenv("PATH", tmp)
	defer func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("PATH", oldPath)
		_ = os.Setenv("NIX_BIN", oldNix)
	}()

	got, err := locateNixBinary()
	if err != nil {
		t.Fatalf("locateNixBinary fallback failed: %v", err)
	}
	if got != nixPath {
		t.Fatalf("expected fallback %q, got %q", nixPath, got)
	}
}

func TestSSHDryRunDoesNotExecute(t *testing.T) {
	tmp := t.TempDir()
	nixPath := filepath.Join(tmp, "fake-nix")
	if err := os.WriteFile(nixPath, []byte("#!/bin/sh\necho test\n"), 0o755); err != nil {
		t.Fatalf("write fake nix failed: %v", err)
	}

	oldNixBin := os.Getenv("NIX_BIN")
	oldRepoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	oldRunner := execRunner

	if err := os.MkdirAll(filepath.Join(tmp, "env"), 0o755); err != nil {
		t.Fatalf("mkdir env failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write dialtone entrypoint failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "env/mesh.json"), []byte(`[{"name":"gold","aliases":["gold"],"host":"10.0.0.1","user":"alice","port":"22"}]`), 0o644); err != nil {
		t.Fatalf("write mesh config failed: %v", err)
	}

	executed := false
	execRunner = func(cmd *exec.Cmd) error {
		executed = true
		return nil
	}

	_ = os.Setenv("NIX_BIN", nixPath)
	_ = os.Setenv("DIALTONE_REPO_ROOT", tmp)
	defer func() {
		_ = os.Setenv("NIX_BIN", oldNixBin)
		_ = os.Setenv("DIALTONE_REPO_ROOT", oldRepoRoot)
		execRunner = oldRunner
	}()

	output := captureStdout(t, func() {
		if err := run([]string{"--host", "gold", "--dry-run", "--command", "echo hi"}); err != nil {
			t.Fatalf("run dry-run failed: %v", err)
		}
	})
	if executed {
		t.Fatal("dry-run should not execute command")
	}
	if !strings.Contains(output, "nix command:") || !strings.Contains(output, "echo hi") {
		t.Fatalf("dry-run output missing expected content: %q", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe failed: %v", err)
	}
	oldStdout := os.Stdout
	os.Stdout = writer

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		t.Fatalf("reading stdout pipe failed: %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("close reader failed: %v", err)
	}
	return buf.String()
}
