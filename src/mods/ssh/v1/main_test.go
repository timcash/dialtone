package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestSSHV1Layout(t *testing.T) {
	root := currentDir(t)
	for _, rel := range []string{
		"README.md",
		"mod.json",
		"main.go",
		"main_test.go",
		filepath.Join("cli", "main.go"),
		filepath.Join("cli", "main_test.go"),
	} {
		if _, err := os.Stat(filepath.Join(root, rel)); err != nil {
			t.Fatalf("expected %s in ssh/v1: %v", rel, err)
		}
	}
}

func TestSSHParseArgs(t *testing.T) {
	opts, err := parseArgs([]string{"--host", "gold", "--user", "user", "--password", "secret", "--port", "2022", "--command", "echo hello", "--dry-run"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if opts.host != "gold" {
		t.Fatalf("expected host gold, got %q", opts.host)
	}
	if opts.user != "user" {
		t.Fatalf("expected user user, got %q", opts.user)
	}
	if opts.password != "secret" {
		t.Fatalf("expected password secret, got %q", opts.password)
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

func TestSSHParseArgsRejectsNixpkgsURL(t *testing.T) {
	_, err := parseArgs([]string{"--host", "gold", "--nixpkgs-url", "nix://example"})
	if err == nil || !strings.Contains(err.Error(), "no longer supported") {
		t.Fatalf("expected nixpkgs-url rejection, got %v", err)
	}
}

func TestSSHParseArgsHostFallbackFromRunPrefix(t *testing.T) {
	tmp := t.TempDir()
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

	oldRepoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	oldActive := os.Getenv("DIALTONE_NIX_ACTIVE")
	oldSSHBin := os.Getenv("DIALTONE_SSH_BIN")
	_ = os.Setenv("DIALTONE_REPO_ROOT", tmp)
	_ = os.Setenv("DIALTONE_NIX_ACTIVE", "1")
	_ = os.Setenv("DIALTONE_SSH_BIN", "/nix/store/test-openssh/bin/ssh")
	defer func() {
		_ = os.Setenv("DIALTONE_REPO_ROOT", oldRepoRoot)
		_ = os.Setenv("DIALTONE_NIX_ACTIVE", oldActive)
		_ = os.Setenv("DIALTONE_SSH_BIN", oldSSHBin)
	}()

	output := captureStdout(t, func() {
		if err := run([]string{"run", "--host", "gold", "--dry-run"}); err != nil {
			t.Fatalf("run with run-prefix failed: %v", err)
		}
	})
	if !strings.Contains(output, "command:") {
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

func TestSSHLoadMeshConfigPrefersDialtoneJSON(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "env"), 0o755); err != nil {
		t.Fatalf("mkdir env failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "dialtone_mod"), []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatalf("write dialtone entrypoint failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "env", "dialtone.json"), []byte(`{"mesh_nodes":[{"name":"grey","aliases":["grey"],"host":"192.168.4.31","user":"user","password":"secret","port":"22"}]}`), 0o644); err != nil {
		t.Fatalf("write dialtone json failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "env", "mesh.json"), []byte(`[{"name":"grey","aliases":["grey"],"host":"wrong.example","user":"tim","port":"22"}]`), 0o644); err != nil {
		t.Fatalf("write mesh json failed: %v", err)
	}

	oldRepoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	_ = os.Setenv("DIALTONE_REPO_ROOT", tmp)
	defer func() {
		_ = os.Setenv("DIALTONE_REPO_ROOT", oldRepoRoot)
	}()

	nodes, err := loadMeshConfig()
	if err != nil {
		t.Fatalf("loadMeshConfig failed: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].User != "user" || nodes[0].Password != "secret" || nodes[0].Host != "192.168.4.31" {
		t.Fatalf("unexpected dialtone node: %+v", nodes[0])
	}
}

func TestSSHBuildCommandUsesNodeDefaults(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	opts, err := parseArgs([]string{"--host", "gold"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	withShellSSH(t, "/nix/store/test-openssh/bin/ssh", func() {
		cmd, err := buildSSHCommand(opts, node)
		if err != nil {
			t.Fatalf("buildSSHCommand failed: %v", err)
		}
		if cmd.Path != "/nix/store/test-openssh/bin/ssh" {
			t.Fatalf("expected nix shell ssh path, got %q", cmd.Path)
		}
		if got := strings.Join(cmd.Args, " "); !strings.Contains(got, "-p 2223") {
			t.Fatalf("expected port 2223 from mesh node, got args %q", got)
		}
		if !strings.HasSuffix(strings.TrimSpace(cmd.Args[len(cmd.Args)-1]), "@example.com") {
			t.Fatalf("expected default user target, got target arg %q", cmd.Args[len(cmd.Args)-1])
		}
	})
}

func TestSSHBuildCommandRespectsOverrides(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	opts, err := parseArgs([]string{"--host", "gold", "--user", "alice", "--port", "2200", "--command", "echo hi"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	withShellSSH(t, "/nix/store/test-openssh/bin/ssh", func() {
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
	})
}

func TestSSHBuildCommandUsesExpectForPasswordAuth(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	opts, err := parseArgs([]string{"--host", "gold", "--user", "alice", "--password", "secret", "--command", "echo hi"})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	withShellSSH(t, "/nix/store/test-openssh/bin/ssh", func() {
		cmd, err := buildSSHCommand(opts, node)
		if err != nil {
			t.Fatalf("buildSSHCommand failed: %v", err)
		}
		if !strings.HasSuffix(cmd.Path, "/expect") && cmd.Path != "expect" {
			t.Fatalf("expected expect wrapper, got %q", cmd.Path)
		}
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, "/nix/store/test-openssh/bin/ssh") {
			t.Fatalf("expected nix ssh path inside expect args, got %q", joined)
		}
		if !strings.Contains(joined, "PreferredAuthentications=password") {
			t.Fatalf("expected password auth args, got %q", joined)
		}
		if strings.Contains(joined, "BatchMode=yes") {
			t.Fatalf("did not expect batch mode for password auth, got %q", joined)
		}
	})
}

func TestSSHBuildCommandUsesNodePasswordByDefault(t *testing.T) {
	node := meshNode{Name: "grey", Host: "192.168.4.31", User: "user", Password: "secret", Port: "22"}
	withShellSSH(t, "/nix/store/test-openssh/bin/ssh", func() {
		cmd, err := buildSSHCommand(sshOptions{}, node)
		if err != nil {
			t.Fatalf("buildSSHCommand failed: %v", err)
		}
		if !strings.HasSuffix(cmd.Path, "/expect") && cmd.Path != "expect" {
			t.Fatalf("expected expect wrapper, got %q", cmd.Path)
		}
		joined := strings.Join(cmd.Args, " ")
		if !strings.Contains(joined, "PreferredAuthentications=password") {
			t.Fatalf("expected password auth args, got %q", joined)
		}
		if !strings.Contains(joined, "user@192.168.4.31") {
			t.Fatalf("expected grey target, got %q", joined)
		}
	})
}

func TestSSHBuildCommandRequiresNixShellSSH(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	oldActive := os.Getenv("DIALTONE_NIX_ACTIVE")
	oldSSHBin := os.Getenv("DIALTONE_SSH_BIN")
	defer func() {
		_ = os.Setenv("DIALTONE_NIX_ACTIVE", oldActive)
		_ = os.Setenv("DIALTONE_SSH_BIN", oldSSHBin)
	}()
	_ = os.Setenv("DIALTONE_NIX_ACTIVE", "")
	_ = os.Setenv("DIALTONE_SSH_BIN", "")

	_, err := buildSSHCommand(sshOptions{}, node)
	if err == nil || !strings.Contains(err.Error(), "must run inside the Dialtone nix shell") {
		t.Fatalf("expected nix shell requirement error, got %v", err)
	}
}

func TestSSHBuildCommandRejectsHostSSHPath(t *testing.T) {
	node := meshNode{Name: "gold", Host: "example.com", User: "user", Port: "2223"}
	withShellSSH(t, "/usr/bin/ssh", func() {
		_, err := buildSSHCommand(sshOptions{}, node)
		if err == nil || !strings.Contains(err.Error(), "requires nix-provided ssh") {
			t.Fatalf("expected nix-provided ssh error, got %v", err)
		}
	})
}

func TestSSHBuildCommandPrefersTailnetHostCandidate(t *testing.T) {
	node := meshNode{
		Name:           "gold",
		Host:           "10.0.0.9",
		HostCandidates: []string{"192.168.1.9", "gold.shad-artichoke.ts.net"},
		User:           "user",
		Port:           "22",
	}
	withShellSSH(t, "/nix/store/test-openssh/bin/ssh", func() {
		cmd, err := buildSSHCommand(sshOptions{}, node)
		if err != nil {
			t.Fatalf("buildSSHCommand failed: %v", err)
		}
		if !strings.HasSuffix(cmd.Args[len(cmd.Args)-1], "user@gold.shad-artichoke.ts.net") {
			t.Fatalf("expected tailnet host to be selected first, got target %q", cmd.Args[len(cmd.Args)-1])
		}
	})
}

func TestSSHDryRunDoesNotExecute(t *testing.T) {
	tmp := t.TempDir()
	oldRepoRoot := os.Getenv("DIALTONE_REPO_ROOT")
	oldActive := os.Getenv("DIALTONE_NIX_ACTIVE")
	oldSSHBin := os.Getenv("DIALTONE_SSH_BIN")
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

	_ = os.Setenv("DIALTONE_REPO_ROOT", tmp)
	_ = os.Setenv("DIALTONE_NIX_ACTIVE", "1")
	_ = os.Setenv("DIALTONE_SSH_BIN", "/nix/store/test-openssh/bin/ssh")
	defer func() {
		_ = os.Setenv("DIALTONE_REPO_ROOT", oldRepoRoot)
		_ = os.Setenv("DIALTONE_NIX_ACTIVE", oldActive)
		_ = os.Setenv("DIALTONE_SSH_BIN", oldSSHBin)
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
	if !strings.Contains(output, "command:") || !strings.Contains(output, "echo hi") {
		t.Fatalf("dry-run output missing expected content: %q", output)
	}
}

func withShellSSH(t *testing.T, sshBin string, fn func()) {
	t.Helper()
	oldActive := os.Getenv("DIALTONE_NIX_ACTIVE")
	oldSSHBin := os.Getenv("DIALTONE_SSH_BIN")
	_ = os.Setenv("DIALTONE_NIX_ACTIVE", "1")
	_ = os.Setenv("DIALTONE_SSH_BIN", sshBin)
	defer func() {
		_ = os.Setenv("DIALTONE_NIX_ACTIVE", oldActive)
		_ = os.Setenv("DIALTONE_SSH_BIN", oldSSHBin)
	}()
	fn()
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

func currentDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Dir(file)
}
