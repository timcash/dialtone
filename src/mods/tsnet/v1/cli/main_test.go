package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTsnetV1CLISmoke(t *testing.T) {
	for _, name := range []string{
		"main.go",
		"bootstrap.go",
		"install.go",
		"keepalive.go",
		"status.go",
		"paths.go",
		"config.go",
		"tsnet_api.go",
		"hosts.go",
	} {
		path := filepath.Join(testDataDir(), name)
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("tsnet v1 cli is missing %s: %v", name, err)
		}
	}

	usage := captureStdout(t, printUsage)
	for _, cmd := range []string{
		"bootstrap",
		"install",
		"keepalive",
		"status",
		"hosts",
	} {
		if !strings.Contains(usage, cmd) {
			t.Fatalf("tsnet v1 usage missing command %q", cmd)
		}
	}

	t.Setenv("DIALTONE_HOSTNAME", "Gold-Host")
	bootstrap := parseBootstrapArgs([]string{})
	if bootstrap.hostname != "Gold-Host" {
		t.Fatalf("bootstrap should default to env hostname, got %q", bootstrap.hostname)
	}

	bootstrap = parseBootstrapArgs([]string{"--host", "Alpha", "--env", "env/test.env", "--state-dir", "state", "--skip-acl", "--prefer-native", "--no-keepalive"})
	if bootstrap.hostname != "Alpha" || bootstrap.stateDir != "state" || !bootstrap.skipACL {
		t.Fatalf("parseBootstrapArgs did not retain flags as expected")
	}
	if bootstrap.noKeepalive != true || bootstrap.preferNative != true {
		t.Fatalf("parseBootstrapArgs did not retain boolean flags as expected")
	}

	keepalive := parseKeepaliveArgs([]string{"--host", "mesh-node", "--state-dir", "/tmp/mesh-state"})
	if keepalive.hostname != "mesh-node" || keepalive.stateDir != "/tmp/mesh-state" {
		t.Fatalf("parseKeepaliveArgs did not capture host/state-dir")
	}

	cfg, err := resolveConfig("", "")
	if err != nil {
		t.Fatalf("resolveConfig should return a usable default config: %v", err)
	}
	if cfg.Hostname == "" || cfg.StateDir == "" || cfg.AuthKeyEnv == "" || cfg.APIKeyEnv == "" {
		t.Fatalf("resolveConfig returned incomplete values: %+v", cfg)
	}

	abs, raw := resolveFilePath("/repo", "", "env/.env")
	if raw != "env/.env" || abs != filepath.Join("/repo", "env/.env") {
		t.Fatalf("resolveFilePath fallback unexpected: path=%q raw=%q", abs, raw)
	}

	abs, raw = resolveFilePath("/repo", "env/other.env", "env/.env")
	if raw != "env/other.env" || abs != filepath.Join("/repo", "env/other.env") {
		t.Fatalf("resolveFilePath should honor explicit relative path: path=%q raw=%q", abs, raw)
	}
}

func TestTsnetV1HostsCommand(t *testing.T) {
	t.Helper()
	prev := tailscaleStatusOutput
	defer func() {
		tailscaleStatusOutput = prev
	}()

	tailscaleStatusOutput = func(args ...string) ([]byte, error) {
		if len(args) != 2 || args[0] != "status" || args[1] != "--json" {
			return nil, fmt.Errorf("unexpected tailscale status command args: %v", args)
		}
		return []byte(`{
			"Self": {"DNSName":"gold.example.com"},
			"Peers": [
				{"DNSName":"gold.example.com"},
				{"HostName":"mesh-alpha"},
				{"DNSName":"mesh-beta.example.com"},
				{"DNSName":"mesh-alpha"}
			]
		}`), nil
	}

	out := captureStdout(t, func() {
		if err := runHosts(nil); err != nil {
			t.Fatalf("runHosts failed: %v", err)
		}
	})
	lines := strings.Fields(strings.TrimSpace(out))
	if len(lines) != 3 {
		t.Fatalf("expected 3 hosts, got %d: %q", len(lines), out)
	}
	expected := []string{"gold.example.com", "mesh-alpha", "mesh-beta.example.com"}
	for i, exp := range expected {
		if lines[i] != exp {
			t.Fatalf("hosts output mismatch at %d: got %q expected %q", i, lines[i], exp)
		}
	}

	parsed, err := parseHostsArgs([]string{})
	if err != nil {
		t.Fatalf("parseHostsArgs with no args should be valid: %v", err)
	}
	if parsed.outputFormat != hostsOutputText {
		t.Fatalf("expected default format text, got %q", parsed.outputFormat)
	}
	_, err = parseHostsArgs([]string{"--bad"})
	if err == nil {
		t.Fatalf("parseHostsArgs should reject positional arguments")
	}
	_, err = parseHostsArgs([]string{"--format", "yaml"})
	if err == nil {
		t.Fatalf("parseHostsArgs should reject unsupported format")
	}
	_, err = parseHostsArgs([]string{"--format"})
	if err == nil {
		t.Fatalf("parseHostsArgs should reject missing format value")
	}
	outCfg, err := parseHostsArgs([]string{"--format", "json"})
	if err != nil {
		t.Fatalf("parseHostsArgs failed for json format: %v", err)
	}
	if outCfg.outputFormat != hostsOutputJSON {
		t.Fatalf("parseHostsArgs returned unexpected format %q", outCfg.outputFormat)
	}
}

func TestTsnetV1HostsCommandJSONOutput(t *testing.T) {
	t.Helper()
	prev := tailscaleStatusOutput
	defer func() {
		tailscaleStatusOutput = prev
	}()

	tailscaleStatusOutput = func(args ...string) ([]byte, error) {
		if len(args) != 2 || args[0] != "status" || args[1] != "--json" {
			return nil, fmt.Errorf("unexpected tailscale status command args: %v", args)
		}
		return []byte(`{
			"Self": {"DNSName":"gold.example.com"},
			"Peers": [
				{"DNSName":"mesh-alpha"},
				{"DNSName":"mesh-beta.example.com"}
			]
		}`), nil
	}

	output := captureStdout(t, func() {
		if err := runHosts([]string{"--format", "json"}); err != nil {
			t.Fatalf("runHosts failed: %v", err)
		}
	})

	var payload struct {
		Hosts []string `json:"hosts"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &payload); err != nil {
		t.Fatalf("failed to parse hosts json output: %v", err)
	}
	if len(payload.Hosts) != 3 {
		t.Fatalf("expected 3 hosts in json payload, got %d: %s", len(payload.Hosts), output)
	}
	expected := []string{"gold.example.com", "mesh-alpha", "mesh-beta.example.com"}
	for i, want := range expected {
		if payload.Hosts[i] != want {
			t.Fatalf("hosts json mismatch at %d: got %q expected %q", i, payload.Hosts[i], want)
		}
	}
}

func TestTsnetV1HostsCommandRemoteSSHJSON(t *testing.T) {
	hosts := parseSSHHostList(os.Getenv("DIALTONE_TEST_HOSTS"))
	if len(hosts) == 0 {
		t.Skip("set DIALTONE_TEST_HOSTS (comma-separated) to enable remote host integration checks")
	}

	for _, host := range hosts {
		h := strings.TrimSpace(host)
		if h == "" {
			continue
		}
		t.Run("host="+h, func(t *testing.T) {
			cmd := "cd /Users/user/dialtone 2>/dev/null || cd /home/user/dialtone 2>/dev/null || cd ~/dialtone; ./dialtone2.sh -- tsnet v1 hosts --format json"
			raw, err := runRemoteCommand(h, cmd)
			if err != nil {
				t.Fatalf("remote tsnet hosts command failed for %s: %v", h, err)
			}

			payload := struct {
				Hosts []string `json:"hosts"`
			}{}
			if err := json.Unmarshal([]byte(extractTrailingJSON(raw)), &payload); err != nil {
				t.Fatalf("failed parsing json for %s: %v\noutput=%s", h, err, raw)
			}
			if len(payload.Hosts) == 0 {
				t.Fatalf("expected hosts for %s, got empty output", h)
			}
		})
	}
}

func TestTsnetV1ParseStatusHosts(t *testing.T) {
	raw := []byte(`{
		"Self": {"DNSName":"gold"},
		"Peer": {
			"mesh-dup": {"DNSName":"mesh-alpha"},
			"mesh-beta": {"HostName":"mesh-beta"}
		},
		"Peers": [
			{"HostName":"mesh-alpha"},
			{"Name":"mesh-charlie"}
		]
	}`)
	hosts, err := parseStatusHosts(raw)
	if err != nil {
		t.Fatalf("parseStatusHosts failed: %v", err)
	}
	if len(hosts) != 4 {
		t.Fatalf("expected deduped list of 4 hosts, got %d: %+v", len(hosts), hosts)
	}
	expected := []string{"gold", "mesh-alpha", "mesh-beta", "mesh-charlie"}
	for i, want := range expected {
		if hosts[i] != want {
			t.Fatalf("sorted host mismatch at %d: got %q expected %q", i, hosts[i], want)
		}
	}
}

func TestTsnetV1ParseStatusHostsRejectsInvalidJSON(t *testing.T) {
	if _, err := parseStatusHosts([]byte(`{bad json`)); err == nil {
		t.Fatalf("parseStatusHosts should reject invalid JSON")
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

func extractTrailingJSON(raw string) string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "{") {
			return line
		}
	}
	return strings.TrimSpace(raw)
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
