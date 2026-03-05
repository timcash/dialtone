package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestMoshV1RemoteNixInstallScript(t *testing.T) {
	out := buildRemoteNixInstallScript("gold")
	if !strings.Contains(out, "set -e") {
		t.Fatalf("remote install script missing set -e")
	}
	if !strings.Contains(out, "/nix/var/nix/profiles/default/bin/nix") {
		t.Fatalf("remote install script missing local nix profile path")
	}
	if !strings.Contains(out, "--extra-experimental-features flakes") {
		t.Fatalf("remote install script missing flakes feature flags")
	}
	if !strings.Contains(out, "nix unavailable on remote target gold; cannot install mosh") {
		t.Fatalf("remote install script missing target-specific error message")
	}
}

func TestMoshV1LocalProfileInstallCommandError(t *testing.T) {
	err := profileInstallMoshWithCommand("/definitely/not/a/real/nix/bin")
	if err == nil {
		t.Fatalf("expected profile install to fail with bad nix path")
	}
	if !strings.Contains(err.Error(), "failed to install mosh locally") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMoshV1ExecutableFallback(t *testing.T) {
	origHome := os.Getenv("HOME")
	tmp := filepath.Join(os.TempDir(), "dialtone-mosh-test-"+t.Name())
	if err := os.RemoveAll(tmp); err != nil {
		t.Fatalf("cleanup temp home failed: %v", err)
	}
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatalf("create temp home failed: %v", err)
	}
	defer func() {
		_ = os.RemoveAll(tmp)
		_ = os.Setenv("HOME", origHome)
	}()
	_ = os.Setenv("HOME", tmp)

	fakeCmd := filepath.Join(tmp, ".nix-profile", "bin", "dialtone-mosh-test-bin")
	if err := os.MkdirAll(filepath.Dir(fakeCmd), 0o755); err != nil {
		t.Fatalf("create fake dir failed: %v", err)
	}
	if err := os.WriteFile(fakeCmd, []byte("#!/bin/sh\necho fake\n"), 0o755); err != nil {
		t.Fatalf("write fake cmd failed: %v", err)
	}

	if !isExecutableAvailable("dialtone-mosh-test-bin") {
		t.Fatalf("expected fallback executable check to detect %q", fakeCmd)
	}
	if isExecutableAvailable("dialtone-mosh-test-bin-missing") {
		t.Fatalf("expected missing executable to be false")
	}
}

func TestMoshV1CLISmoke(t *testing.T) {
	for _, name := range []string{
		"main.go",
		"install.go",
		"setup.go",
		"paths.go",
		"connect.go",
	} {
		path := filepath.Join(testDataDir(), name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("mosh v1 cli is missing %s: %v", name, err)
		}
	}

	usage := captureStdout(t, printUsage)
	for _, cmd := range []string{
		"install",
		"setup",
		"connect",
	} {
		if !strings.Contains(usage, cmd) {
			t.Fatalf("mosh v1 usage missing command %q", cmd)
		}
	}

	installOpts := parseInstallArgs([]string{"--nixpkgs-url", "nix://example"})
	if installOpts.nixpkgsURL != "nix://example" {
		t.Fatalf("install parser did not capture --nixpkgs-url")
	}
	if installOpts.ensure {
		t.Fatalf("install parser default ensure should be false")
	}

	installOpts = parseInstallArgs([]string{"--nixpkgs-url", "nix://example", "--ensure"})
	if installOpts.nixpkgsURL != "nix://example" {
		t.Fatalf("install parser did not preserve --nixpkgs-url with --ensure")
	}
	if !installOpts.ensure {
		t.Fatalf("install parser did not capture --ensure")
	}

	setup := parseSetupArgs([]string{})
	if setup.host != "" || setup.ensure {
		t.Fatalf("empty setup args should yield defaults (host empty, ensure false)")
	}

	setup = parseSetupArgs([]string{"--host", "Gold", "--ensure"})
	if setup.host != "Gold" || !setup.ensure {
		t.Fatalf("setup parser did not capture host/ensure")
	}

	connect := parseConnectArgs([]string{"--host", "Gold", "--ensure", "--session", "dialtone-gold", "--command", "tmux list-sessions", "--repo-root", "/tmp/repo", "--fallback-ssh", "--dry-run"})
	if connect.host != "Gold" {
		t.Fatalf("connect parser did not capture host")
	}
	if !connect.ensure {
		t.Fatalf("connect parser did not capture --ensure")
	}
	if !connect.fallbackSSH {
		t.Fatalf("connect parser did not capture --fallback-ssh")
	}
	if !connect.dryRun {
		t.Fatalf("connect parser did not capture --dry-run")
	}
	if connect.session != "dialtone-gold" {
		t.Fatalf("connect parser did not capture session")
	}
	if connect.command != "tmux list-sessions" {
		t.Fatalf("connect parser did not capture command: %q", connect.command)
	}
	if connect.repoRoot != "/tmp/repo" {
		t.Fatalf("connect parser did not capture repo root: %q", connect.repoRoot)
	}

	connect = parseConnectArgs([]string{"--command", "echo hello", "rover-1"})
	if connect.host != "rover-1" {
		t.Fatalf("connect parser should use positional host argument")
	}
	if connect.command != "echo hello" {
		t.Fatalf("connect parser positional test command wrong")
	}
}

func TestMoshV1ResolveConnectRepoRoot(t *testing.T) {
	abs, err := resolveConnectRepoRoot("/tmp/repo")
	if err != nil {
		t.Fatalf("resolveConnectRepoRoot failed with absolute path: %v", err)
	}
	if abs != "/tmp/repo" {
		t.Fatalf("absolute repo root should be preserved, got %q", abs)
	}

	absRel, err := resolveConnectRepoRoot("tmp/repo")
	if err != nil {
		t.Fatalf("resolveConnectRepoRoot failed with relative path: %v", err)
	}
	if strings.TrimSpace(absRel) == "" {
		t.Fatalf("resolveConnectRepoRoot returned empty path")
	}
	if !strings.HasPrefix(absRel, "/") {
		t.Fatalf("resolveConnectRepoRoot should return absolute path for relative input: %q", absRel)
	}

	empty, err := resolveConnectRepoRoot("")
	if err != nil {
		t.Fatalf("resolveConnectRepoRoot failed for empty input: %v", err)
	}
	if empty != "" {
		t.Fatalf("resolveConnectRepoRoot should return empty when no explicit path provided: %q", empty)
	}
}

func TestMoshV1ConnectDryRun(t *testing.T) {
	prevRunner := moshConnectRunner
	prevSSH := sshConnectRunner
	prevSetup := setupRunner
	defer func() {
		moshConnectRunner = prevRunner
		sshConnectRunner = prevSSH
		setupRunner = prevSetup
	}()

	moshCalled := false
	sshCalled := false
	setupCalled := false
	moshConnectRunner = func(host, remoteShell string) error {
		moshCalled = true
		return nil
	}
	sshConnectRunner = func(host, remoteShell string) error {
		sshCalled = true
		return nil
	}
	setupRunner = func(args []string) error {
		setupCalled = true
		return nil
	}

	output := captureStdout(t, func() {
		if err := runConnect([]string{"--host", "gold", "--repo-root", "/tmp", "--dry-run"}); err != nil {
			t.Fatalf("runConnect dry-run failed: %v", err)
		}
	})
	if moshCalled {
		t.Fatalf("dry-run should not call mosh")
	}
	if sshCalled {
		t.Fatalf("dry-run should not call ssh")
	}
	if setupCalled {
		t.Fatalf("dry-run should not call setup")
	}
	if !strings.Contains(output, "export DIALTONE_HOSTNAME='gold'") {
		t.Fatalf("dry-run output missing host export: %q", output)
	}
	if !strings.Contains(output, "tmux new-session -A -s 'dialtone-gold'") {
		t.Fatalf("dry-run output missing default tmux command: %q", output)
	}
}

func TestMoshV1ConnectFallbackToSSH(t *testing.T) {
	prevRunner := moshConnectRunner
	prevSSH := sshConnectRunner
	prevSetup := setupRunner
	defer func() {
		moshConnectRunner = prevRunner
		sshConnectRunner = prevSSH
		setupRunner = prevSetup
	}()

	moshCalled := false
	sshCalled := false
	session := "dialtone-gold"

	moshConnectRunner = func(host, command string) error {
		moshCalled = true
		return errors.New("mosh unavailable")
	}
	sshConnectRunner = func(host, command string) error {
		if !moshCalled {
			t.Fatalf("ssh should only run if mosh fails")
		}
		sshCalled = true
		if host != "gold" {
			t.Fatalf("ssh host mismatch: %q", host)
		}
		if !strings.Contains(command, "tmux new-session -A -s") {
			t.Fatalf("ssh command did not include tmux fallback: %q", command)
		}
		if !strings.Contains(command, session) {
			t.Fatalf("ssh command did not include session name: %q", command)
		}
		return nil
	}
	setupRunner = func(args []string) error {
		if strings.TrimSpace(strings.Join(args, " ")) != "--host gold --ensure" {
			t.Fatalf("setup not called with expected args: %v", args)
		}
		return nil
	}

	if err := runConnect([]string{"--host", "gold", "--ensure", "--fallback-ssh"}); err != nil {
		t.Fatalf("runConnect fallback scenario failed: %v", err)
	}
	if !moshCalled {
		t.Fatalf("expected mosh attempt")
	}
	if !sshCalled {
		t.Fatalf("expected ssh fallback")
	}
}

func TestMoshV1BuildRemoteShellCommand(t *testing.T) {
	cmd := buildRemoteShellCommand("tmux list-sessions", "Gold", "/tmp/repo")
	if !strings.Contains(cmd, "export DIALTONE_HOSTNAME='gold'") {
		t.Fatalf("buildRemoteShellCommand should set hostname")
	}
	if !strings.Contains(cmd, "cd '/tmp/repo'") {
		t.Fatalf("buildRemoteShellCommand should include repo root")
	}
	if !strings.Contains(cmd, "tmux list-sessions") {
		t.Fatalf("buildRemoteShellCommand should include command body")
	}
}

func TestMoshV1RemoteConnectDryRunSSH(t *testing.T) {
	hosts := parseSSHHostList(os.Getenv("DIALTONE_TEST_HOSTS"))
	if len(hosts) == 0 {
		t.Skip("set DIALTONE_TEST_HOSTS (comma-separated) to enable remote mosh integration checks")
	}

	for _, host := range hosts {
		h := strings.TrimSpace(host)
		if h == "" {
			continue
		}
		t.Run("host="+h, func(t *testing.T) {
			cmd := "cd /Users/user/dialtone 2>/dev/null || cd /home/user/dialtone 2>/dev/null || cd ~/dialtone; ./dialtone_mod -- mosh v1 connect --host " + h + " --dry-run"
			output, err := runRemoteCommand(h, cmd)
			if err != nil {
				t.Fatalf("remote mosh connect dry run failed for %s: %v", h, err)
			}
			if !strings.Contains(output, "export DIALTONE_HOSTNAME='") {
				t.Fatalf("remote output missing host export for %s: %s", h, output)
			}
			if !strings.Contains(output, "tmux new-session -A -s 'dialtone-") {
				t.Fatalf("remote output missing tmux command for %s: %s", h, output)
			}
		})
	}
}

func runRemoteCommand(host, command string) (string, error) {
	cmd := exec.Command(
		"ssh",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		host,
		command,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return strings.TrimSpace(string(output)), fmt.Errorf("ssh command failed: %v output=%q", err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

func parseSSHHostList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func testDataDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("unable to locate test source file")
	}
	return filepath.Dir(file)
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
